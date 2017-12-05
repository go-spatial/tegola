package gpkg

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths/points"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util"
	"github.com/terranodo/tegola/util/dict"
	"github.com/terranodo/tegola/wkb"

	"bytes"
	"context"
	"fmt"
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
	util.CodeLogger.Debug("Attempting gpkg.Layers()")
	layerCount := len(p.layers)
	ls := make([]mvt.LayerInfo, layerCount)

	i := 0
	for _, player := range p.layers {
		ls[i] = player
		i++
	}

	util.CodeLogger.Debugf("Ok, returning mvt.LayerInfo array: %v", ls)
	return ls, nil
}

func (p *GPKGProvider) MVTLayer(ctx context.Context, layerName string, tile tegola.TegolaTile, dtags map[string]interface{}) (*mvt.Layer, error) {
	util.CodeLogger.Debugf("GPKGProvider MVTLayer() called for %v", layerName)
	filepath := p.FilePath

	// Check that layer is within bounding box
	var layerBBox points.BoundingBox
	layerBBox = p.layers[layerName].bbox

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
			util.CodeLogger.Fatal("DefaultSRID != tegola.WebMercator requires changes here")
		}
		tileBBox = tileBBox.ConvertSrid(tegola.WebMercator, p.layers[layerName].srid)
	}

	if layerBBox.DisjointBB(tileBBox) {
		msg := fmt.Sprintf("Layer '%v' bounding box %v is outside tile bounding box %v, "+
			"will not load any features", layerName, layerBBox, tileBBox)
		util.CodeLogger.Debugf(msg)
		return new(mvt.Layer), nil
	}

	db, err := getGpkgConnection(filepath)
	if err != nil {
		return nil, err
	}
	defer releaseGpkgConnection(filepath)

	geomTablename := p.layers[layerName].tablename
	geomFieldname := p.layers[layerName].geomFieldname
	idFieldname := p.layers[layerName].idFieldname
	// Get all feature rows for the layer requested.
	rtreeTablename := fmt.Sprintf("rtree_%v_geom", geomTablename)
	// l - layer table, si - spatial index
	selectClause := fmt.Sprintf("SELECT `%v` AS fid, `%v` AS geom", idFieldname, geomFieldname)
	for _, tf := range p.layers[layerName].tagFieldnames {
		selectClause += fmt.Sprintf(", `%v`", tf)
	}
	qtext := fmt.Sprintf("%v FROM %v l JOIN %v si ON l.%v = si.id WHERE geom IS NOT NULL "+
		"AND NOT (si.minx > ? OR si.maxx < ? OR si.miny > ? OR si.maxy < ?);",
		selectClause, geomTablename, rtreeTablename, idFieldname)
	qparams := []interface{}{tileBBox[2], tileBBox[0], tileBBox[3], tileBBox[1]}
	util.CodeLogger.Debugf("qtext: %v\nqparams: %v\n", qtext, qparams)
	rows, err := db.Query(qtext, qparams...)
	if err != nil {
		util.CodeLogger.Errorf("Error during query: %v (%v)- %v", qtext, qparams, err)
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := 0; i < len(cols); i++ {
		valPtrs[i] = &vals[i]
	}

	pLayer := p.layers[layerName]
	newLayer := new(mvt.Layer)
	newLayer.Name = layerName

	rowCount := 0
	var geom tegola.Geometry
	for rows.Next() {
		// Copy default tags to feature tag map
		ftags := make(map[string]interface{})
		for k, v := range dtags {
			ftags[k] = v
		}

		geom = nil
		rowCount++
		err = rows.Scan(valPtrs...)
		if err != nil {
			util.CodeLogger.Error(err)
			continue
		}
		var fid uint64

		for i := 0; i < len(cols); i++ {
			if vals[i] == nil {
				continue
			} else if cols[i] == idFieldname {
				fid = uint64(vals[i].(int64))
			} else if cols[i] == geomFieldname {
				util.CodeLogger.Debugf("Doing gpkg geometry extraction...", vals[i])
				var h GeoPackageBinaryHeader
				geomData := vals[i].([]byte)
				h.Init(geomData)

				reader := bytes.NewReader(geomData[h.Size():])
				geom, err = wkb.Decode(reader)

				if err != nil {
					util.CodeLogger.Errorf("Error decoding geometry: %v", err)
				}

				if h.SRSId() != DefaultSRID {
					util.CodeLogger.Infof("SRID %v != %v, trying to convert...", pLayer.srid, DefaultSRID)
					// We need to convert our points to Webmercator.
					g, err := basic.ToWebMercator(pLayer.srid, geom)
					if err != nil {
						util.CodeLogger.Errorf(
							"Was unable to transform geometry to webmercator from "+
								"SRID (%v) for layer (%v) due to error: %v",
							pLayer.srid, layerName, err)
						return nil, err
					} else {
						util.CodeLogger.Info("...conversion ok")
					}
					geom = g.Geometry
				} else {
					util.CodeLogger.Infof("SRID already default (%v), no conversion necessary", DefaultSRID)
				}
			} else {
				// Grab any non-nil, non-id, & non-geometry column as a tag
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
					util.CodeLogger.Error(err)
				}
			}
		}

		if geom == nil {
			util.CodeLogger.Warn("No geometry in row, skipping feature")
			continue
		}

		f := mvt.Feature{
			ID:       &fid,
			Tags:     ftags,
			Geometry: geom,
		}
		newLayer.AddFeatures(f)
	}

	if rowCount != len(newLayer.Features()) {
		util.CodeLogger.Errorf("newLayer feature count doesn't match table row count (%v != %v)\n",
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
		util.CodeLogger.Error(err)
		return nil, err
	}
	err := fmt.Errorf("Execution should not leave switch block.")
	util.CodeLogger.Fatal(err)
	return nil, err
}

func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	util.CodeLogger.Info("GPKGProvider NewProvider() called with config: %v\n", config)
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
			err := fmt.Errorf("'name' is required for a feature's config.")
			util.CodeLogger.Fatal(err)
		}
		if layerConfig["tablename"] == nil && layerConfig["sql"] == nil {
			err := fmt.Errorf("Either 'tablename' or 'sql' is required for a feature's config.")
			util.CodeLogger.Fatal(err)
		}
		if layerConfig["tablename"] != nil && layerConfig["sql"] != nil {
			err := fmt.Errorf("Only one of 'tablename', 'sql' may appear in a layer's config.")
			util.CodeLogger.Fatal(err)
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
		util.CodeLogger.Error(msg)
		return nil, err
	}

	util.CodeLogger.Debugf("Opening gpkg at: %v", filepath)
	db, err := getGpkgConnection(filepath)
	if err != nil {
		util.CodeLogger.Errorf("Error opening gpkg file: %v", err)
		return nil, err
	}
	defer releaseGpkgConnection(filepath)

	p := GPKGProvider{FilePath: filepath, layers: make(map[string]GPKGLayer)}

	qtext := "SELECT c.table_name, c.min_x, c.min_y, c.max_x, c.max_y, c.srs_id, " +
		"gc.column_name, gc.geometry_type_name " +
		"FROM gpkg_contents c JOIN gpkg_geometry_columns gc ON c.table_name == gc.table_name " +
		"WHERE c.data_type = 'features';"
	rows, err := db.Query(qtext)
	if err != nil {
		util.CodeLogger.Errorf("Error during query: %v - %v", qtext, err)
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
			util.CodeLogger.Errorf(
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
			// Layer from custom sql
			l = GPKGLayer{
				name: layerName,
				sql:  layerConfig["sql"].(string),
			}
		}
		p.layers[layerName] = l
	}

	return &p, err
}

func init() {
	provider.Register(ProviderName, NewProvider)
}
