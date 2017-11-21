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

	_ "github.com/mattn/go-sqlite3"

	"database/sql"

	"bytes"
	"context"
	"fmt"
)

const (
	ProviderName = "gpkg"
	FilePath     = "FilePath"
	DefaultSRID  = tegola.WebMercator
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
	name     string
	geomType tegola.Geometry
	srid     int
	// Bounding box containing all features in the layer: [minX, minY, maxX, maxY]
	bbox points.BoundingBox
	sql  string
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
		msg := "Layer %v is outside tile bounding box, will not load any features"
		util.CodeLogger.Debugf(msg, layerName)
		return new(mvt.Layer), nil
	}

	util.CodeLogger.Infof("Opening gpkg at: ", filepath)
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	// Get all feature rows for the layer requested.
	rtreeTablename := fmt.Sprintf("rtree_%v_geom", layerName)
	// l - layer table, si - spatial index
	qtext := fmt.Sprintf("SELECT * FROM %v l JOIN %v si ON l.fid = si.id WHERE geom IS NOT NULL "+
		"AND NOT (si.minx > ? OR si.maxx < ? OR si.miny > ? OR si.maxy < ?);",
		layerName, rtreeTablename)

	qparams := []interface{}{tileBBox[2], tileBBox[0], tileBBox[3], tileBBox[1]}
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
		var gid uint64

		for i := 0; i < len(cols); i++ {
			if vals[i] == nil {
				continue
			} else if cols[i] == "geom" {
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
				// Grab any non-nil & non-geometry column as a tag
				switch v := vals[i].(type) {
				case []uint8:
					asBytes := make([]byte, len(v)+1)
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
			ID:       &gid,
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

func getGeomColumnDetails(db *sql.DB) (map[string]*GeomColumn, error) {
	// Returns a map with table name (string) as key and struct containing column details as value
	columnDetails := make(map[string]*GeomColumn)

	sqlText := "SELECT table_name, column_name, geometry_type_name, srs_id FROM gpkg_geometry_columns;"
	rows, err := db.Query(sqlText)
	defer rows.Close()

	if err != nil {
		util.CodeLogger.Errorf("Error in query collecting geometry column details: %v", err)
		return nil, err
	}

	for rows.Next() {
		var tablename string
		col := new(GeomColumn)
		rows.Scan(&tablename, &((*col).name), &(*col).geometryType, &(*col).srsId)
		// http://www.geopackage.org/spec/#geometry_types
		switch col.geometryType {
		case "POINT":
			col.tegolaGeometry = new(basic.Point)
		case "LINESTRING":
			col.tegolaGeometry = new(basic.Line)
		case "POLYGON":
			col.tegolaGeometry = new(basic.Polygon)
		case "MULTIPOINT":
			col.tegolaGeometry = new(basic.MultiPoint)
		case "MULTILINESTRING":
			col.tegolaGeometry = new(basic.MultiLine)
		case "MULTIPOLYGON":
			col.tegolaGeometry = new(basic.MultiPolygon)
		default:
			err := fmt.Errorf("Unsupported gpkg geometry type: %v\n", col.geometryType)
			util.CodeLogger.Error(err)
		}
		columnDetails[tablename] = col
	}

	return columnDetails, nil
}

func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	m := dict.M(config)
	filepath, err := m.String("FilePath", nil)
	if filepath == "" || err != nil {
		msg := fmt.Sprintf("Bad gpkg filepath: %v", filepath)
		if err != nil {
			msg += fmt.Sprintf(" error: %v\n", err)
		}
		util.CodeLogger.Error(msg)
		return nil, err
	}

	util.CodeLogger.Debugf("Opening gpkg at: %v", filepath)
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		util.CodeLogger.Errorf("Error opening gpkg file: %v", err)
		return nil, err
	}

	p := GPKGProvider{FilePath: filepath, layers: make(map[string]GPKGLayer)}

	qtext := "SELECT * FROM gpkg_contents"
	rows, err := db.Query(qtext)
	if err != nil {
		util.CodeLogger.Errorf("Error during query: %v - %v", qtext, err)
		return nil, err
	}
	defer rows.Close()

	var tablename string
	var dataType string
	var identifier string
	var description string
	var lastChange string
	var minX float64
	var minY float64
	var maxX float64
	var maxY float64
	var srid int

	logMsg := "gpkg_contents: "

	geomColumnDetails, err := getGeomColumnDetails(db)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		rows.Scan(&tablename, &dataType, &identifier, &description, &lastChange,
			&minX, &minY, &maxX, &maxY, &srid)

		// Get layer geometry as tegola geometry instance corresponding to dataType text for table
		layerQuery := fmt.Sprintf("SELECT * FROM %v;", tablename)
		colDetails := geomColumnDetails[tablename]
		bbox := [4]float64{minX, minY, maxX, maxY}
		p.layers[tablename] = GPKGLayer{
			name: tablename, sql: layerQuery, geomType: colDetails.tegolaGeometry, srid: srid,
			bbox: bbox}

		var logMsgPart string
		fmt.Sprintf(logMsgPart, "(%v-%i) ", tablename, srid)
		logMsg += logMsgPart
	}
	util.CodeLogger.Debug(logMsg)

	return &p, err
}

func init() {
	provider.Register(ProviderName, NewProvider)
}
