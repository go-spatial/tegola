// +build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

const (
	Name                 = "gpkg"
	DefaultSRID          = tegola.WebMercator
	DefaultIDFieldName   = "fid"
	DefaultGeomFieldName = "geom"
)

// config keys
const (
	ConfigKeyFilePath       = "filepath"
	ConfigKeyLayers         = "layers"
	ConfigKeyLayerName      = "name"
	ConfigKeyTableName      = "tablename"
	ConfigKeySQL            = "sql"
	ConfigKeyGeomIDField    = "id_fieldname"
	ConfigKeyStartTimeField = "tstart"
	ConfigKeyEndTimeField   = "tend"
	ConfigKeyFields         = "fields"
)

func decodeGeometry(bytes []byte) (*BinaryHeader, geom.Geometry, error) {
	h, err := NewBinaryHeader(bytes)
	if err != nil {
		log.Error("error decoding geometry header: %v", err)
		return h, nil, err
	}

	geo, err := wkb.DecodeBytes(bytes[h.Size():])
	if err != nil {
		log.Error("error decoding geometry: %v", err)
		return h, nil, err
	}

	return h, geo, nil
}

type Provider struct {
	// path to the geopackage file
	Filepath string
	// map of layer name and corresponding sql
	layers map[string]Layer
	// reference to the database connection
	db *sql.DB
}

func (p *Provider) Layers() ([]provider.LayerInfo, error) {
	log.Debug("attempting gpkg.Layers()")

	ls := make([]provider.LayerInfo, len(p.layers))

	var i int
	for _, player := range p.layers {
		ls[i] = player
		i++
	}

	log.Debugf("returning LayerInfo array: %v", ls)

	return ls, nil
}

func (p *Provider) TileFeatures(ctx context.Context, layer string, tile provider.Tile, fn func(f *provider.Feature) error) error {
	log.Debugf("fetching layer %v", layer)

	pLayer := p.layers[layer]

	// read the tile extent
	tileBBox, tileSRID := tile.BufferedExtent()

	// TODO(arolek): reimplement once the geom package has reprojection
	// check if the SRID of the layer differs from that of the tile. tileSRID is assumed to always be WebMercator
	if pLayer.srid != tileSRID {
		minGeo, err := basic.FromWebMercator(pLayer.srid, basic.Point{tileBBox.MinX(), tileBBox.MinY()})
		if err != nil {
			return fmt.Errorf("error converting point: %v ", err)
		}

		maxGeo, err := basic.FromWebMercator(pLayer.srid, basic.Point{tileBBox.MaxX(), tileBBox.MaxY()})
		if err != nil {
			return fmt.Errorf("error converting point: %v ", err)
		}

		tileBBox = &geom.Extent{
			minGeo.AsPoint().X(), minGeo.AsPoint().Y(),
			maxGeo.AsPoint().X(), maxGeo.AsPoint().Y(),
		}
	}

	var qtext string

	if pLayer.tablename != "" {
		// If layer was specified via "tablename" in config, construct query.
		rtreeTablename := fmt.Sprintf("rtree_%v_geom", pLayer.tablename)

		selectClause := fmt.Sprintf("SELECT `%v` AS fid, `%v` AS geom", pLayer.idFieldname, pLayer.geomFieldname)

		for _, tf := range pLayer.tagFieldnames {
			selectClause += fmt.Sprintf(", `%v`", tf)
		}

		// l - layer table, si - spatial index
		qtext = fmt.Sprintf("%v FROM %v l JOIN %v si ON l.%v = si.id WHERE geom IS NOT NULL AND !BBOX! ORDER BY %v", selectClause, pLayer.tablename, rtreeTablename, pLayer.idFieldname, pLayer.idFieldname)

		z, _, _ := tile.ZXY()
		qtext = replaceTokens(qtext, &z, tileBBox)
	} else {
		// If layer was specified via "sql" in config, collect it
		z, _, _ := tile.ZXY()
		qtext = replaceTokens(pLayer.sql, &z, tileBBox)
	}

	log.Debugf("qtext: %v", qtext)

	rows, err := p.db.Query(qtext)
	if err != nil {
		log.Errorf("err during query: %v - %v", qtext, err)
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	for rows.Next() {
		// check if the context cancelled or timed out
		if ctx.Err() != nil {
			return ctx.Err()
		}

		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := 0; i < len(cols); i++ {
			valPtrs[i] = &vals[i]
		}

		if err = rows.Scan(valPtrs...); err != nil {
			log.Errorf("err reading row values: %v", err)
			return err
		}

		feature := provider.Feature{
			Properties: map[string]interface{}{},
		}

		for i := range cols {
			// check if the context cancelled or timed out
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if vals[i] == nil {
				continue
			}

			switch cols[i] {
			case pLayer.idFieldname:
				feature.ID, err = provider.ConvertFeatureID(vals[i])
				if err != nil {
					return err
				}

			case pLayer.geomFieldname:
				log.Debugf("extracting geopackage geometry header.", vals[i])

				geomData, ok := vals[i].([]byte)
				if !ok {
					log.Errorf("unexpected column type for geom field. got %t", vals[i])
					return errors.New("unexpected column type for geom field. expected blob")
				}

				h, geo, err := decodeGeometry(geomData)
				if err != nil {
					return err
				}

				feature.SRID = uint64(h.SRSId())
				feature.Geometry = geo

			case "minx", "miny", "maxx", "maxy", "min_zoom", "max_zoom":
				// Skip these columns used for bounding box and zoom filtering
				continue

			default:
				// Grab any non-nil, non-id, non-bounding box, & non-geometry column as a tag
				switch v := vals[i].(type) {
				case []uint8:
					asBytes := make([]byte, len(v))
					for j := 0; j < len(v); j++ {
						asBytes[j] = v[j]
					}

					feature.Properties[cols[i]] = string(asBytes)
				case int64:
					feature.Properties[cols[i]] = v
				default:
					// TODO(arolek): return this error?
					log.Errorf("unexpected type for sqlite column data: %v: %T", cols[i], v)
				}
			}
		}

		// pass the feature to the provided call back
		if err = fn(&feature); err != nil {
			return err
		}
	}

	return nil
}

func (p *Provider) SupportedFilters() []string {
	return []string{
		// TODO:(jivan) --- Commented-out filterers aren't yet implemented
		provider.TimeFiltererType,
		provider.ExtentFiltererType,
		provider.IndexFiltererType,
		provider.PropertyFiltererType,
	}
}

func (p *Provider) StreamFeatures(ctx context.Context, layer string, zoom uint,
	fn provider.FeatureConsumer, filters ...provider.BaseFilterer) error {
	log.Debugf("fetching layer %v", layer)

	var tileBBox *geom.Extent // geom.MinMaxer
	var indices []uint
	var tp provider.TimeFilterer
	var props map[string]interface{}
	// cast supplied filters to typed filters and collect their values
	for _, f := range filters {
		if tf, ok := f.(provider.ExtentFilterer); ok {
			e := tf.Extent()
			tileBBox = &e
		} else if tf, ok := f.(provider.IndexFilterer); ok {
			indices = make([]uint, 2)
			indices[0] = tf.Start()
			indices[1] = tf.End()
		} else if tf, ok := f.(provider.TimeFilterer); ok {
			tp = tf
		} else if tf, ok := f.(provider.PropertyFilterer); ok {
			props = tf.Map()
		} else {
			return fmt.Errorf("unexpected filter: (%T) %v", f, f)
		}
	}

	pLayer := p.layers[layer]

	var qtext string

	var indexClause string
	if indices != nil {
		indexClause = fmt.Sprintf("LIMIT %v OFFSET %v", indices[1]-indices[0], indices[0])
	}

	var timePeriodClause string = "NULL IS NULL"
	if tp != nil && (!tp.Start().IsZero() || !tp.End().IsZero()) {
		// We'll treat a zero start value as negative infinity & zero end value as infinity
		// We've got values for both start & end
		if !tp.Start().IsZero() && !tp.End().IsZero() {
			// query period contains entire layer period
			tpclause1 := fmt.Sprintf(
				"'%v' < %v AND '%v' > %v", tp.Start(), pLayer.tstartFieldname, tp.End(), pLayer.tendFieldname)
			// query period begins in layer period
			tpclause2 := fmt.Sprintf(
				"'%v' >= %v AND '%v' <= %v", tp.Start(), pLayer.tstartFieldname, tp.Start(), pLayer.tendFieldname)
			// query period ends in layer period
			tpclause3 := fmt.Sprintf(
				"'%v' >= %v AND '%v' <= %v", tp.End(), pLayer.tstartFieldname, tp.End(), pLayer.tendFieldname)
			timePeriodClause = fmt.Sprintf("((%v) OR (%v) OR (%v))", tpclause1, tpclause2, tpclause3)
		} else if !tp.Start().IsZero() { // We've only got a start for the period
			// query period starts in or before layer period
			timePeriodClause = fmt.Sprintf("'%v' <= '%v'", tp.Start(), pLayer.tendFieldname)
		} else if !tp.End().IsZero() {
			// query period ends in or after layer period
			timePeriodClause = fmt.Sprintf("'%v' >= '%v'", tp.End(), pLayer.tstartFieldname)
		}
	}

	var propertyClause string = "NULL IS NULL"
	if props != nil {
		pcs := []string{}
		for k, v := range props {
			switch v.(type) {
			case string:
				v = fmt.Sprintf("'%v'", v)
			}
			pcs = append(pcs, fmt.Sprintf("`%v` = %v", k, v))
		}
		propertyClause = "(" + strings.Join(pcs, " AND ") + ")"
	}

	if pLayer.tablename != "" {
		// If layer was specified via "tablename" in config, construct query.
		rtreeTablename := fmt.Sprintf("rtree_%v_geom", pLayer.tablename)

		selectClause := fmt.Sprintf("SELECT `%v` AS fid, `%v` AS geom", pLayer.idFieldname, pLayer.geomFieldname)

		for _, tf := range pLayer.tagFieldnames {
			selectClause += fmt.Sprintf(", `%v`", tf)
		}

		// l - layer table, si - spatial index

		qtext = fmt.Sprintf(`
			%v
			FROM %v l JOIN %v si ON l.%v = si.id
			WHERE geom IS NOT NULL AND !BBOX!
				AND %v AND %v
			ORDER BY %v
			%v`,
			selectClause,
			pLayer.tablename, rtreeTablename, pLayer.idFieldname,
			timePeriodClause, propertyClause,
			pLayer.idFieldname,
			indexClause)

		qtext = replaceTokens(qtext, &zoom, tileBBox)
	} else {
		// If layer was specified via "sql" in config, collect it
		qtext = replaceTokens(pLayer.sql, &zoom, tileBBox)
		// Add ORDER BY, LIMIT, OFFSET to query
		qtext = strings.TrimSpace(qtext)
		qtlen := len(qtext)
		if []rune(qtext)[qtlen-1] == ';' {
			qtext = qtext[:qtlen-1]
		}

		if strings.Contains(strings.ToUpper(qtext), "WHERE") {
			if strings.Count(qtext, "WHERE") > 1 {
				return fmt.Errorf("SQL too complicated for current implementation (multiple WHERE clauses found)")
			}
			strings.Replace(qtext, "WHERE", fmt.Sprintf("WHERE %v AND %v AND", timePeriodClause, propertyClause), -1)
		} else {
			qtext = qtext + fmt.Sprintf(" WHERE %v AND %v", timePeriodClause, propertyClause)
		}

		qtext = qtext + fmt.Sprintf(" ORDER BY %v %v", pLayer.idFieldname, indexClause)
	}

	log.Debugf("qtext: %v", qtext)
	rows, err := p.db.Query(qtext)
	if err != nil {
		log.Errorf("err during query: %v - %v", qtext, err)
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	for rows.Next() {
		// check if the context cancelled or timed out
		if ctx.Err() != nil {
			return ctx.Err()
		}

		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := 0; i < len(cols); i++ {
			valPtrs[i] = &vals[i]
		}

		if err = rows.Scan(valPtrs...); err != nil {
			log.Errorf("err reading row values: %v", err)
			return err
		}

		feature := provider.Feature{
			Properties: map[string]interface{}{},
		}

		for i := range cols {
			// check if the context cancelled or timed out
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if vals[i] == nil {
				continue
			}

			switch cols[i] {
			case pLayer.idFieldname:
				feature.ID, err = provider.ConvertFeatureID(vals[i])
				if err != nil {
					return err
				}

			case pLayer.geomFieldname:
				log.Debugf("extracting geopackage geometry header.", vals[i])

				geomData, ok := vals[i].([]byte)
				if !ok {
					log.Errorf("unexpected column type for geom field. got %t", vals[i])
					return errors.New("unexpected column type for geom field. expected blob")
				}

				h, geo, err := decodeGeometry(geomData)
				if err != nil {
					return err
				}

				feature.SRID = uint64(h.SRSId())
				feature.Geometry = geo

			case "minx", "miny", "maxx", "maxy", "min_zoom", "max_zoom":
				// Skip these columns used for bounding box and zoom filtering
				continue

			default:
				// Grab any non-nil, non-id, non-bounding box, & non-geometry column as a tag
				switch v := vals[i].(type) {
				case []uint8:
					asBytes := make([]byte, len(v))
					for j := 0; j < len(v); j++ {
						asBytes[j] = v[j]
					}

					feature.Properties[cols[i]] = string(asBytes)
				case int64:
					feature.Properties[cols[i]] = v
				case time.Time:
					feature.Properties[cols[i]] = v
				default:
					// TODO(arolek): return this error?
					log.Errorf("unexpected type for sqlite column data: %v: %T", cols[i], v)
				}
			}
		}

		// pass the feature to the provided call back
		if err = fn(&feature); err != nil {
			return err
		}
	}

	return nil
}

// Close will close the Provider's database connection
func (p *Provider) Close() error {
	return p.db.Close()
}

type GeomTableDetails struct {
	geomFieldname string
	geomType      geom.Geometry
	srid          uint64
	bbox          geom.Extent
}

type GeomColumn struct {
	name         string
	geometryType string
	geom         geom.Geometry // to populate Layer.geomType
	srsId        int
}

func geomNameToGeom(name string) (geom.Geometry, error) {
	switch name {
	case "POINT":
		return geom.Point{}, nil
	case "LINESTRING":
		return geom.LineString{}, nil
	case "POLYGON":
		return geom.Polygon{}, nil
	case "MULTIPOINT":
		return geom.MultiPoint{}, nil
	case "MULTILINESTRING":
		return geom.MultiLineString{}, nil
	case "MULTIPOLYGON":
		return geom.MultiPolygon{}, nil
	}

	return nil, fmt.Errorf("unsupported geometry type: %v", name)
}
