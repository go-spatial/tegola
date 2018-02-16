// +build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding/wkb"
	"github.com/terranodo/tegola/internal/log"
	"github.com/terranodo/tegola/provider"
)

const (
	Name                 = "gpkg"
	DefaultSRID          = tegola.WebMercator
	DefaultIDFieldName   = "fid"
	DefaultGeomFieldName = "geom"
)

//	config keys
const (
	ConfigKeyFilePath    = "filepath"
	ConfigKeyLayers      = "layers"
	ConfigKeyLayerName   = "name"
	ConfigKeyTableName   = "tablename"
	ConfigKeySQL         = "sql"
	ConfigKeyGeomIDField = "id_fieldname"
	ConfigKeyFields      = "fields"
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
	// map of layer name and corrosponding sql
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

	//	read the tile extent
	bufferedExtent, tileSRID := tile.BufferedExtent()

	// TODO: leverage minx/y maxx/y methods once the BufferedExtent returns a geom.Extent type
	tileBBox := geom.BoundingBox{
		{bufferedExtent[0][0], bufferedExtent[0][1]}, //minx, miny
		{bufferedExtent[1][0], bufferedExtent[1][1]}, //maxx, maxy
	}

	// TODO(arolek): reimplement once the geom package has reprojection
	// check if the SRID of the layer differs from that of the tile. tileSRID is assumed to always be WebMercator
	if pLayer.srid != tileSRID {
		minGeo, err := basic.FromWebMercator(pLayer.srid, basic.Point{bufferedExtent[0][0], bufferedExtent[0][1]})
		if err != nil {
			return fmt.Errorf("error converting point: %v ", err)
		}

		maxGeo, err := basic.FromWebMercator(pLayer.srid, basic.Point{bufferedExtent[1][0], bufferedExtent[1][1]})
		if err != nil {
			return fmt.Errorf("error converting point: %v ", err)
		}

		tileBBox = geom.BoundingBox{
			{minGeo.AsPoint().X(), minGeo.AsPoint().Y()},
			{maxGeo.AsPoint().X(), maxGeo.AsPoint().Y()},
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
		qtext = fmt.Sprintf("%v FROM %v l JOIN %v si ON l.%v = si.id WHERE geom IS NOT NULL AND !BBOX!", selectClause, pLayer.tablename, rtreeTablename, pLayer.idFieldname)

		z, _, _ := tile.ZXY()
		qtext = replaceTokens(qtext, z, tileBBox)
	} else {
		// If layer was specified via "sql" in config, collect it
		z, _, _ := tile.ZXY()
		qtext = replaceTokens(pLayer.sql, z, tileBBox)
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
			Tags: map[string]interface{}{},
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

					feature.Tags[cols[i]] = string(asBytes)
				case int64:
					feature.Tags[cols[i]] = v
				default:
					// TODO(arolek): return this error?
					log.Errorf("unexpected type for sqlite column data: %v: %T", cols[i], v)
				}
			}
		}

		//	pass the feature to the provided call back
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
	bbox          geom.BoundingBox
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
