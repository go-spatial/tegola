package gpkg

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding/wkb"
	"github.com/terranodo/tegola/internal/log"
	"github.com/terranodo/tegola/maths/points"
	"github.com/terranodo/tegola/provider"
	"github.com/terranodo/tegola/util/dict"
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

func init() {
	provider.Register(ProviderName, NewTileProvider)
}

func NewTileProvider(config map[string]interface{}) (provider.Tiler, error) {
	//	parse our config
	m := dict.M(config)

	filepath, err := m.String(ConfigKeyFilePath, nil)
	if err != nil {
		return nil, err
	}
	if filepath == "" {
		return nil, ErrInvalidFilePath{filepath}
	}

	db, err := GetConnection(filepath)
	if err != nil {
		log.Error("gpkg: error opening gpkg file: %v", err)
		return nil, err
	}
	defer ReleaseConnection(filepath)

	p := Provider{
		Filepath: filepath,
		layers:   make(map[string]Layer),
	}

	//	this query is used to read the metadata from the gpkg_contents table for tables that have geometry fields
	qtext := `
		SELECT
			c.table_name, c.min_x, c.min_y, c.max_x, c.max_y, c.srs_id, gc.column_name, gc.geometry_type_name
		FROM
			gpkg_contents c JOIN gpkg_geometry_columns gc ON c.table_name == gc.table_name
		WHERE
			c.data_type = 'features';`

	rows, err := db.Query(qtext)
	if err != nil {
		log.Errorf("gpgk: error during query: %v - %v", qtext, err)
		return nil, err
	}
	defer rows.Close()

	//	container for tracking metadata for each table with a geometry
	geomTableDetails := make(map[string]GeomTableDetails)

	//	iterate each row extracting meta data about each table
	for rows.Next() {
		var tablename, geomCol, geomType sql.NullString
		var minX, minY, maxX, maxY sql.NullFloat64
		var srid sql.NullInt64

		if err = rows.Scan(&tablename, &minX, &minY, &maxX, &maxY, &srid, &geomCol, &geomType); err != nil {
			return nil, err
		}

		// map the returned geom type to a tegola geom type
		tg, err := geomNameToGeom(geomType.String)
		if err != nil {
			log.Error("gpkg: error mapping geom type (%v): %v", geomType, err)
			return nil, err
		}

		geomTableDetails[tablename.String] = GeomTableDetails{
			geomFieldname: geomCol.String,
			geomType:      tg,
			srid:          uint64(srid.Int64),
			//	the extent of the layer's features
			bbox: points.BoundingBox{minX.Float64, minY.Float64, maxX.Float64, maxY.Float64},
		}
	}

	layers, ok := config[ConfigKeyLayers].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("gpkg: expected %v to be a []map[string]interface{}", ConfigKeyLayers)
	}

	// TODO(arolek): check for layers configured multiple times
	// lyrsSeen := make(map[string]int)
	for i, v := range layers {

		layerConf := dict.M(v)

		layerName, err := layerConf.String(ConfigKeyLayerName, nil)
		if err != nil {
			return nil, fmt.Errorf("gpkg: for layer (%v) we got the following error trying to get the layer's name field: %v", i, err)
		}
		if layerName == "" {
			return nil, ErrMissingLayerName
		}

		if layerConf[ConfigKeyTableName] == nil && layerConf[ConfigKeySQL] == nil {
			return nil, errors.New("gokg: 'tablename' or 'sql' is required for a feature's config")
		}

		if layerConf[ConfigKeyTableName] != nil && layerConf[ConfigKeySQL] != nil {
			return nil, errors.New("gokg: 'tablename' or 'sql' is required for a feature's config. you have both")
		}

		idFieldname := DEFAULT_ID_FIELDNAME
		idFieldname, err = layerConf.String(ConfigKeyGeomIDField, &idFieldname)
		if err != nil {
			return nil, fmt.Errorf("gpkg: for layer (%v) %v : %v", i, layerName, err)
		}

		tagFieldnames, err := layerConf.StringSlice(ConfigKeyFields)
		if err != nil {
			return nil, fmt.Errorf("gpkg: for layer (%v) %v %v field had the following error: %v", i, layerName, ConfigKeyFields, err)
		}

		//	layer container. will be added to the provider after it's configured
		layer := Layer{
			name: layerName,
		}

		if layerConf[ConfigKeyTableName] != nil {
			tablename, err := layerConf.String(ConfigKeyTableName, &idFieldname)
			if err != nil {
				return nil, fmt.Errorf("gpkg: for layer (%v) %v : %v", i, layerName, err)
			}

			layer.tablename = tablename
			layer.tagFieldnames = tagFieldnames
			layer.geomFieldname = geomTableDetails[tablename].geomFieldname
			layer.geomType = geomTableDetails[tablename].geomType
			layer.idFieldname = idFieldname
			layer.srid = geomTableDetails[tablename].srid
			layer.bbox = geomTableDetails[tablename].bbox

		} else {
			var customSQL string
			customSQL, err = layerConf.String(ConfigKeySQL, &customSQL)
			if err != nil {
				return nil, fmt.Errorf("gpkg: for %v layer(%v) %v has an error: %v", i, layerName, ConfigKeySQL, err)
			}
			layer.sql = customSQL

			customSQL, tokensPresent := replaceTokens(customSQL)

			// Get geometry type & srid from geometry of first row.
			qtext := fmt.Sprintf("SELECT geom FROM (%v) LIMIT 1;", customSQL)

			// Set bounds & zoom params to include all layers
			// Bounds checks need params: maxx, minx, maxy, miny
			// TODO(arolek): this assumes WGS84. should be more flexible
			qparams := []interface{}{float64(180.0), float64(-180.0), float64(85.0511), float64(-85.0511)}

			if tokensPresent["ZOOM"] {
				// min_zoom will always be less than 100, and max_zoom will always be greater than 0.
				qparams = append(qparams, 100, 0)
			}
			log.Debugf("gpgk: qtext: %v, params: %v", qtext, qparams)

			row := db.QueryRow(qtext, qparams...)

			var geomData []byte
			err = row.Scan(&geomData)
			if err == sql.ErrNoRows {
				log.Warnf("gpkg: layer '%v' with custom SQL has 0 rows, skipping: %v", layerName, customSQL)
				continue
			} else if err != nil {
				// TODO(arolek): why are we not returning here?
				log.Errorf("gpkg: layer '%v' problem executing custom SQL, skipping: %v", layerName, err)
				continue
			}

			var h BinaryHeader
			h.Init(geomData)

			reader := bytes.NewReader(geomData[h.Size():])

			layer.geomType, err = wkb.Decode(reader)
			if err != nil {
				return nil, fmt.Errorf("gpkg: error extracting geometry: %v", err)
			}

			layer.srid = uint64(h.SRSId())
			layer.geomFieldname = DEFAULT_GEOM_FIELDNAME
			layer.idFieldname = DEFAULT_ID_FIELDNAME
		}

		p.layers[layer.name] = layer
	}

	return &p, err
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

		// TODO(aroke): there has to be a cleaner way to do this but rows.Scan() does not like []interface{} at run time and throws the error
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
			if vals[i] == nil {
				continue
			}

			switch cols[i] {
			case pLayer.idFieldname:
				// TODO(arolek): check for error? assertions are dangerous unless we're 100% sure it will always be this type
				feature.ID = uint64(vals[i].(int64))
			case pLayer.geomFieldname:
				log.Debugf("gpkg: extracting geopackage geometry header.", vals[i])

				// TODO(arolek): check for error? assertions are dangerous unless we're 100% sure it will always be this type
				geomData := vals[i].([]byte)
				geomHeader := new(BinaryHeader)
				geomHeader.Init(geomData)

				feature.SRID = uint64(geomHeader.SRSId())

				feature.Geometry, err = wkb.DecodeBytes(geomData[geomHeader.Size():])
				if err != nil {
					log.Error("gpkg: error decoding geometry: %v", err)
					return err
				}
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
