package gpkg

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/internal/log"
	"github.com/terranodo/tegola/maths/points"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util/dict"
	"github.com/terranodo/tegola/wkb"

	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	ProviderName           = "gpkg"
	FilePath               = "FilePath"
	DefaultSRID            = tegola.WebMercator
	DEFAULT_ID_FIELDNAME   = "fid"
	DEFAULT_GEOM_FIELDNAME = "geom"
)

type GPKGProvider struct {
	mvt.Provider
	// Currently just the path to the gpkg file.
	FilePath string
	// map of layer name and corrosponding sql
	layers map[string]GPKGLayer
}

type GPKGLayer struct {
	mvt.LayerInfo
	name          string
	tablename     string
	features      []string
	tagFieldnames []string
	idFieldname   string
	geomFieldname string
	geomType      tegola.Geometry
	srid          int
	// Bounding box containing all features in the layer: [minX, minY, maxX, maxY]
	bbox points.BoundingBox
	sql  string
}

type GPKGGeomTableDetails struct {
	geomFieldname string
	geomType      tegola.Geometry
	srid          int
	bbox          points.BoundingBox
}

func (l GPKGLayer) Name() string              { return l.name }
func (l GPKGLayer) GeomType() tegola.Geometry { return l.geomType }
func (l GPKGLayer) SRID() int                 { return l.srid }
func (l GPKGLayer) BBox() [4]float64          { return l.bbox }

func (p *GPKGProvider) Layers() ([]mvt.LayerInfo, error) {
	log.Debug("Attempting gpkg.Layers()")
	layerCount := len(p.layers)
	ls := make([]mvt.LayerInfo, layerCount)

	i := 0
	for _, player := range p.layers {
		ls[i] = player
		i++
	}

	log.Debug("Ok, returning mvt.LayerInfo array: %v", ls)
	return ls, nil
}

func replaceTokens(qtext string) (ttext string, tokensPresent map[string]bool) {
	// --- Convert tokens provided to SQL
	// The BBOX token requires parameters ordered as [maxx, minx, maxy, miny] and checks for overlap.
	// 	Until support for named parameters, we'll only support one BBOX token per query.
	// The ZOOM token requires two parameters, both filled with the current zoom level.
	//	Until support for named parameters, the ZOOM token must follow the BBOX token.
	ttext = string(qtext)
	tokensPresent = make(map[string]bool)

	if strings.Count(ttext, "!BBOX!") > 0 {
		tokensPresent["BBOX"] = true
		ttext = strings.Replace(qtext, "!BBOX!", "minx <= ? AND maxx >= ? AND miny <= ? AND maxy >= ?", 1)
	}

	if strings.Count(ttext, "!ZOOM!") > 0 {
		tokensPresent["ZOOM"] = true
		ttext = strings.Replace(ttext, "!ZOOM!", "min_zoom <= ? AND max_zoom >= ?", 1)
	}

	return ttext, tokensPresent
}

func layerFromQuery(ctx context.Context, pLayer *GPKGLayer, rows *sql.Rows, rowCount *int, dtags map[string]interface{}) (
	layer *mvt.Layer, err error) {

	layer = new(mvt.Layer)
	layer.Name = pLayer.Name()

	idFieldname := pLayer.idFieldname
	geomFieldname := pLayer.geomFieldname

	var geom tegola.Geometry

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := 0; i < len(cols); i++ {
		valPtrs[i] = &vals[i]
	}

	// Populates the "features" property of "layer"
	for rows.Next() {
		// Check if the context cancelled or timed out.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		*rowCount++
		// Copy default tags to kick off this feature's tags
		ftags := make(map[string]interface{})
		for k, v := range dtags {
			ftags[k] = v
		}

		geom = nil
		err := rows.Scan(valPtrs...)
		if err != nil {
			log.Error(err)
			continue
		}
		var fid uint64

		for i := 0; i < len(cols); i++ {
			if vals[i] == nil {
				continue
			}

			switch cols[i] {
			case idFieldname:
				fid = uint64(vals[i].(int64))
			case geomFieldname:
				log.Debug("Doing gpkg geometry extraction...", vals[i])
				var h GeoPackageBinaryHeader
				geomData := vals[i].([]byte)
				h.Init(geomData)

				reader := bytes.NewReader(geomData[h.Size():])
				geom, err = wkb.Decode(reader)

				if err != nil {
					log.Error("Error decoding geometry: %v", err)
				}

				if h.SRSId() != DefaultSRID {
					log.Debug("SRID %v != %v, trying to convert...", pLayer.srid, DefaultSRID)
					// We need to convert our points to Webmercator.
					g, err := basic.ToWebMercator(pLayer.srid, geom)
					if err != nil {
						log.Error(
							"Was unable to transform geometry to webmercator from "+
								"SRID (%v) for layer (%v) due to error: %v",
							pLayer.srid, layer.Name, err)
						return nil, err
					} else {
						log.Debug("...conversion ok")
					}
					geom = g.Geometry
				} else {
					log.Info("SRID already default (%v), no conversion necessary", DefaultSRID)
				}
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

					asString := string(asBytes)
					ftags[cols[i]] = asString
				case int64:
					ftags[cols[i]] = v
				default:
					err := fmt.Errorf("Unexpected type for sqlite column data: %v: %T\n", cols[i], v)
					log.Error(err)
				}
			}
		}

		if geom == nil {
			log.Warn("No geometry in row, skipping feature")
			continue
		}

		f := mvt.Feature{
			ID:       &fid,
			Tags:     ftags,
			Geometry: geom,
		}
		layer.AddFeatures(f)
	}
	return layer, nil
}

func (p *GPKGProvider) MVTLayer(ctx context.Context, layerName string, tile tegola.TegolaTile, dtags map[string]interface{}) (*mvt.Layer, error) {
	log.Debug("GPKGProvider MVTLayer() called for %v", layerName)
	filepath := p.FilePath

	// In DefaultSRID (web mercator - 3857)
	tileBBoxStruct := tile.BoundingBox()
	// TODO: There's some confusion between pixel coordinates & WebMercator positions in the tile
	//	bounding box, making the smallest y-value tileBBoxStruct.Maxy and the largest Miny.
	//	Hacking here to ensure a correct bounding box.
	//	At some point, clean up this problem: https://github.com/terranodo/tegola/issues/189
	tileBBox := points.BoundingBox{tileBBoxStruct.Minx, tileBBoxStruct.Maxy,
		tileBBoxStruct.Maxx, tileBBoxStruct.Miny}

	// Convert tile bounding box to gpkg geometry if necessary.
	layerSRID := p.layers[layerName].srid
	if layerSRID != DefaultSRID {
		if DefaultSRID != tegola.WebMercator {
			log.Fatal("DefaultSRID != tegola.WebMercator requires changes here")
		}
		tileBBox = tileBBox.ConvertSrid(tegola.WebMercator, p.layers[layerName].srid)
	}

	// GPKG tables have a bounding box not available to custom queries.
	if p.layers[layerName].tablename != "" {
		// Check that layer is within bounding box
		var layerBBox points.BoundingBox
		layerBBox = p.layers[layerName].bbox

		if layerBBox.DisjointBB(tileBBox) {
			msg := fmt.Sprintf("Layer '%v' bounding box %v is outside tile bounding box %v, "+
				"will not load any features", layerName, layerBBox, tileBBox)
			log.Debug(msg)
			return new(mvt.Layer), nil
		}
	}

	db, err := GetGpkgConnection(filepath)
	if err != nil {
		return nil, err
	}
	defer ReleaseGpkgConnection(filepath)

	var qtext string
	geomFieldname := p.layers[layerName].geomFieldname
	idFieldname := p.layers[layerName].idFieldname

	var tokensPresent map[string]bool
	if p.layers[layerName].tablename != "" {
		// If layer was specified via "tablename" in config, construct query.
		geomTablename := p.layers[layerName].tablename
		rtreeTablename := fmt.Sprintf("rtree_%v_geom", geomTablename)
		// l - layer table, si - spatial index
		selectClause := fmt.Sprintf("SELECT `%v` AS fid, `%v` AS geom", idFieldname, geomFieldname)
		for _, tf := range p.layers[layerName].tagFieldnames {
			selectClause += fmt.Sprintf(", `%v`", tf)
		}
		qtext = fmt.Sprintf("%v FROM %v l JOIN %v si ON l.%v = si.id WHERE geom IS NOT NULL AND !BBOX!",
			selectClause, geomTablename, rtreeTablename, idFieldname)
		qtext, tokensPresent = replaceTokens(qtext)
	} else {
		// If layer was specified via "sql" in config, collect it.
		qtext = p.layers[layerName].sql
		qtext, tokensPresent = replaceTokens(qtext)
	}

	qparams := []interface{}{tileBBox[2], tileBBox[0], tileBBox[3], tileBBox[1]}
	if tokensPresent["ZOOM"] {
		// Add the zoom level, once for comparison to min, once for max.
		qparams = append(qparams, tile.ZLevel(), tile.ZLevel())
	}
	log.Debug("qtext: %v\nqparams: %v\n", qtext, qparams)
	rows, err := db.Query(qtext, qparams...)
	if err != nil {
		log.Error("Error during query: %v (%v)- %v", qtext, qparams, err)
		return nil, err
	}
	defer rows.Close()

	pLayer := p.layers[layerName]
	rowCount := 0
	newLayer, err := layerFromQuery(ctx, &pLayer, rows, &rowCount, dtags)
	if err != nil {
		log.Error("Problem in layerFromQuery(): %v", err)
		return nil, err
	}

	if rowCount != len(newLayer.Features()) {
		log.Error("newLayer feature count doesn't match table row count (%v != %v)\n",
			len(newLayer.Features()), rowCount)
	}

	return newLayer, nil
}

type GeomColumn struct {
	name           string
	geometryType   string
	tegolaGeometry tegola.Geometry // to populate GPKGLayer.geomType
	srsId          int
}

func gpkgGeomNameToTegolaGeometry(geomName string) (tegola.Geometry, error) {
	// Returns a map with table name (string) as key and struct containing column details as value
	switch geomName {
	case "POINT":
		return new(basic.Point), nil
	case "LINESTRING":
		return new(basic.Line), nil
	case "POLYGON":
		return new(basic.Polygon), nil
	case "MULTIPOINT":
		return new(basic.MultiPoint), nil
	case "MULTILINESTRING":
		return new(basic.MultiLine), nil
	case "MULTIPOLYGON":
		return new(basic.MultiPolygon), nil
	default:
		err := fmt.Errorf("Unsupported gpkg geometry type: %v\n", geomName)
		log.Error(err)
		return nil, err
	}
	err := fmt.Errorf("Execution should not leave switch block.")
	log.Fatal(err)
	return nil, err
}

func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	log.Info("GPKGProvider NewProvider() called with config: %v\n", config)
	m := dict.M(config)
	filepath, err := m.String("FilePath", nil)

	layerConfigByName := make(map[string]map[string]interface{})
	var layerConfigs []map[string]interface{}
	if m["layers"] == nil {
		layerConfigs = make([]map[string]interface{}, 0)
	} else {
		layerConfigs = m["layers"].([]map[string]interface{})
	}

	for _, layerConfig := range layerConfigs {
		layerName := layerConfig["name"]
		if layerName == nil {
			log.Fatal("'name' is required for a feature's config.")
		}
		if layerConfig["tablename"] == nil && layerConfig["sql"] == nil {
			log.Fatal("Either 'tablename' or 'sql' is required for a feature's config.")
		}
		if layerConfig["tablename"] != nil && layerConfig["sql"] != nil {
			log.Fatal("Only one of 'tablename', 'sql' may appear in a layer's config.")
		}

		configMap := make(map[string]interface{})
		for key, value := range layerConfig {
			if key == "name" {
				continue
			} else {
				configMap[key] = value
			}
		}

		layerConfigByName[layerName.(string)] = configMap
	}

	if filepath == "" || err != nil {
		msg := fmt.Sprintf("Bad gpkg filepath: %v", filepath)
		if err != nil {
			msg += fmt.Sprintf(" error: %v\n", err)
		}
		log.Error(msg)
		return nil, err
	}

	log.Debug("Opening gpkg at: %v", filepath)
	db, err := GetGpkgConnection(filepath)
	if err != nil {
		log.Error("Error opening gpkg file: %v", err)
		return nil, err
	}
	defer ReleaseGpkgConnection(filepath)

	p := GPKGProvider{FilePath: filepath, layers: make(map[string]GPKGLayer)}

	qtext := "SELECT c.table_name, c.min_x, c.min_y, c.max_x, c.max_y, c.srs_id, " +
		"gc.column_name, gc.geometry_type_name " +
		"FROM gpkg_contents c JOIN gpkg_geometry_columns gc ON c.table_name == gc.table_name " +
		"WHERE c.data_type = 'features';"
	rows, err := db.Query(qtext)
	if err != nil {
		log.Error("Error during query: %v - %v", qtext, err)
		return nil, err
	}
	defer rows.Close()

	var tablename, geomColName, geomTypeName string
	var minX, minY, maxX, maxY float64
	var srid int

	geomTableDetails := make(map[string]GPKGGeomTableDetails)
	for rows.Next() {
		rows.Scan(&tablename, &minX, &minY, &maxX, &maxY, &srid, &geomColName, &geomTypeName)

		// Get layer geometry as tegola geometry instance corresponding to dataType text for table
		tg, err := gpkgGeomNameToTegolaGeometry(geomTypeName)
		if err != nil {
			log.Error(
				"Problem getting geometry type %v as tegola.Geometry: %v", geomTypeName, err)
			return nil, err
		}
		bbox := points.BoundingBox{minX, minY, maxX, maxY}
		geomTableDetails[tablename] = GPKGGeomTableDetails{
			geomFieldname: geomColName, geomType: tg, srid: srid, bbox: bbox}
	}

	for layerName, layerConfig := range layerConfigByName {
		var l GPKGLayer

		var idFieldname string
		if layerConfig["id_fieldname"] == nil {
			idFieldname = DEFAULT_ID_FIELDNAME // "fid"
		} else {
			idFieldname = layerConfig["id_fieldname"].(string)
		}

		tagFieldnames := make([]string, 0)
		if layerConfig["fields"] != nil {
			// TODO: I'm not sure why the value coming out of the config isn't consistent, but it
			//	shouldn't require converting from two different types.
			iArray, ok := layerConfig["fields"].([]interface{})
			if ok {
				for i := 0; i < len(iArray); i++ {
					tagFieldnames = append(tagFieldnames, iArray[i].(string))
				}
			} else if sArray, ok := layerConfig["fields"].([]string); ok {
				tagFieldnames = sArray
			}
		}

		if layerConfig["tablename"] != nil {
			tablename := layerConfig["tablename"].(string)
			l = GPKGLayer{
				name:          layerName,
				tablename:     tablename,
				tagFieldnames: tagFieldnames,
				geomFieldname: geomTableDetails[tablename].geomFieldname,
				geomType:      geomTableDetails[tablename].geomType,
				idFieldname:   idFieldname,
				srid:          geomTableDetails[tablename].srid,
				bbox:          geomTableDetails[tablename].bbox,
			}
		} else {
			// --- Layer from custom sql
			customSqlTemplate := layerConfig["sql"].(string)
			customSql, tokensPresent := replaceTokens(customSqlTemplate)

			// Get geometry type & srid from geometry of first row.
			qtext := fmt.Sprintf("SELECT geom FROM (%v) LIMIT 1;", customSql)
			var geomData []byte
			// Set bounds & zoom params to include all layers
			// Bounds checks need params: maxx, minx, maxy, miny
			qparams := []interface{}{float64(180.0), float64(-180.0), float64(85.0511), float64(-85.0511)}
			if tokensPresent["ZOOM"] {
				// min_zoom will always be less than 100, and max_zoom will always be greater than 0.
				qparams = append(qparams, 100, 0)
			}
			log.Debug("qtext: %v, params: %v", qtext, qparams)
			row := db.QueryRow(qtext, qparams...)
			err = row.Scan(&geomData)
			if err == sql.ErrNoRows {
				log.Warn("Layer '%v' with custom SQL has 0 rows, skipping: %v", layerName, customSql)
				continue
			} else if err != nil {
				log.Error("Layer '%v' problem executing custom SQL, skipping: %v",
					layerName, err)
				continue
			}
			var h GeoPackageBinaryHeader
			h.Init(geomData)
			reader := bytes.NewReader(geomData[h.Size():])
			geom, err := wkb.Decode(reader)
			if err != nil {
				log.Error("Problem extracting gpkg geometry: %v", err)
			}

			l = GPKGLayer{
				name:          layerName,
				sql:           customSqlTemplate,
				srid:          int(h.SRSId()),
				geomType:      geom,
				geomFieldname: "geom",
				idFieldname:   "fid",
			}
		}
		p.layers[layerName] = l
	}

	return &p, err
}

func init() {
	provider.Register(ProviderName, NewProvider)
}
