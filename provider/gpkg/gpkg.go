// +build cgo

package gpkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding/wkb"
	"github.com/terranodo/tegola/internal/log"
	"github.com/terranodo/tegola/maths/points"
	"github.com/terranodo/tegola/provider"
)

const (
	ProviderName           = "gpkg"
	DefaultSRID            = tegola.WebMercator
	DEFAULT_ID_FIELDNAME   = "fid"
	DEFAULT_GEOM_FIELDNAME = "geom"
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
		log.Error("gpkg: error decoding geometry header: %v", err)
		return h, nil, err
	}
	geo, err := wkb.DecodeBytes(bytes[h.Size():])
	if err != nil {
		log.Error("gpkg: error decoding geometry: %v", err)
		return h, nil, err
	}
	return h, geo, nil
}

type Provider struct {
	// path to the geopackage file
	Filepath string
	// map of layer name and corrosponding sql
	layers map[string]Layer
}

func (p *Provider) Layers() ([]provider.LayerInfo, error) {
	log.Debug("gpkg: attempting gpkg.Layers()")

	ls := make([]provider.LayerInfo, len(p.layers))

	var i int
	for _, player := range p.layers {
		ls[i] = player
		i++
	}

	log.Debugf("gpkg: returning LayerInfo array: %v", ls)

	return ls, nil
}

func (p *Provider) TileFeatures(ctx context.Context, layer string, tile provider.Tile, fn func(f *provider.Feature) error) error {
	log.Debugf("gpkg: fetching layer %v", layer)

	pLayer := p.layers[layer]

	// In DefaultSRID (web mercator - 3857)
	// TODO (arolek): support converting the extent to support projections besides web mercator
	extent, tileSRID := tile.BufferedExtent()

	// TODO: There's some confusion between pixel coordinates & WebMercator positions in the tile
	// bounding box, making the smallest y-value tileBBoxStruct.Maxy and the largest Miny.
	// Hacking here to ensure a correct bounding box.
	// At some point, clean up this problem: https://github.com/terranodo/tegola/issues/189
	tileBBox := points.BoundingBox{
		extent[0][0], extent[1][1], //minx, maxy
		extent[1][0], extent[0][1], //maxx, miny
	}

	// check if the SRID of the layer differes from that of the tile. tileSRID is assumed to always be WebMercator
	if pLayer.srid != tileSRID {
		tileBBox = tileBBox.ConvertSRID(tileSRID, pLayer.srid)
	}

	// GPKG tables have a bounding box not available to custom queries.
	if pLayer.tablename != "" {
		// Check that layer is within bounding box
		if pLayer.bbox.DisjointBB(tileBBox) {
			log.Debugf("gpkg: layer '%v' bounding box %v is outside tile bounding box %v, will not load any features", layer, pLayer.bbox, tileBBox)
			return nil
		}
	}

	db, err := GetConnection(p.Filepath)
	if err != nil {
		return err
	}
	defer ReleaseConnection(p.Filepath)

	var qtext string
	var tokensPresent map[string]bool

	if pLayer.tablename != "" {
		// If layer was specified via "tablename" in config, construct query.
		rtreeTablename := fmt.Sprintf("rtree_%v_geom", pLayer.tablename)

		selectClause := fmt.Sprintf("SELECT `%v` AS fid, `%v` AS geom", pLayer.idFieldname, pLayer.geomFieldname)

		for _, tf := range pLayer.tagFieldnames {
			selectClause += fmt.Sprintf(", `%v`", tf)
		}

		// l - layer table, si - spatial index
		qtext = fmt.Sprintf("%v FROM %v l JOIN %v si ON l.%v = si.id WHERE geom IS NOT NULL AND !BBOX!", selectClause, pLayer.tablename, rtreeTablename, pLayer.idFieldname)

		qtext, tokensPresent = replaceTokens(qtext)
	} else {
		// If layer was specified via "sql" in config, collect it
		qtext, tokensPresent = replaceTokens(pLayer.sql)
	}

	// TODO(arolek): implement extent and use MinX/Y MaxX/Y methods
	qparams := []interface{}{tileBBox[2], tileBBox[0], tileBBox[3], tileBBox[1]}

	if tokensPresent["ZOOM"] {
		// Add the zoom level, once for comparison to min, once for max.
		z, _, _ := tile.ZXY()
		qparams = append(qparams, z, z)
	}

	log.Debugf("qtext: %v\nqparams: %v\n", qtext, qparams)

	rows, err := db.Query(qtext, qparams...)
	if err != nil {
		log.Errorf("gpkg: err during query: %v (%v) - %v", qtext, qparams, err)
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

		// TODO(arolek): there has to be a cleaner way to do this but rows.Scan() does not like []interface{} at run time and throws the error
		// "Scan error on column index 0: destination not a pointer"
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := 0; i < len(cols); i++ {
			valPtrs[i] = &vals[i]
		}

		if err = rows.Scan(valPtrs...); err != nil {
			log.Errorf("gpkg: err reading row values: %v", err)
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
				// TODO(arolek): check for error? assertions are dangerous unless we're 100% sure it will always be this type
				feature.ID = uint64(vals[i].(int64))
			case pLayer.geomFieldname:
				log.Debugf("gpkg: extracting geopackage geometry header.", vals[i])

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

			// TODO(arolek): this seems like a bad idea. these could be configured by the user for other purposes
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
					log.Errorf("gpkg: unexpected type for sqlite column data: %v: %T\n", cols[i], v)
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

type GeomTableDetails struct {
	geomFieldname string
	geomType      geom.Geometry
	srid          uint64
	bbox          points.BoundingBox
}

type GeomColumn struct {
	name           string
	geometryType   string
	tegolaGeometry geom.Geometry // to populate Layer.geomType
	srsId          int
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

	return nil, fmt.Errorf("gpkg: unsupported geometry type: %v", name)
}
