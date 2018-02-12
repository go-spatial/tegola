// +build cgo

package gpkg

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/terranodo/tegola/internal/log"
	"github.com/terranodo/tegola/maths/points"
	"github.com/terranodo/tegola/provider"
	"github.com/terranodo/tegola/util/dict"
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

			h, geo, err := decodeGeometry(geomData)
			if err != nil {
				return nil, err
			}
			layer.geomType = geo
			layer.srid = uint64(h.SRSId())
			layer.geomFieldname = DEFAULT_GEOM_FIELDNAME
			layer.idFieldname = DEFAULT_ID_FIELDNAME
		}

		p.layers[layer.name] = layer
	}

	return &p, err
}
