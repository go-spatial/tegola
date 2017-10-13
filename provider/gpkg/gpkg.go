package gpkg

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util/dict"
	"context"
	"errors"
//	"reflect"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	
	"fmt"
)

const Name = "gpkg"
const (
	FilePath = "FilePath"
)

// layer holds information about a query.
// Currently stolen exactly from provider.postgis.layer
type layer struct {
	// The Name of the layer
	name string
	// The SQL to use when querying PostGIS for this layer
	sql string
	// The ID field name, this will default to 'gid' if not set to something other then empty string.
	idField string
	// The Geometery field name, this will default to 'geom' if not set to soemthing other then empty string.
	geomField string
	// GeomType is the the type of geometry returned from the SQL
	geomType tegola.Geometry
	// The SRID that the data in the table is stored in. This will default to WebMercator
	srid int
}

type GPKGProvider struct {
	config string
	// map of layer name and corrosponding sql
	layers map[string]layer
	srid   int
}

type LayerInfo interface {
	Name() string
	GeomType() tegola.Geometry
	SRID() int
}

// Implements mvt.LayerInfo interface
type GPKGLayer struct {
	name string
	geomtype tegola.Geometry
	srid int
}

func(l GPKGLayer) Name() (string) {return l.name}
func(l GPKGLayer) GeomType() (tegola.Geometry) {return l.geomtype}
func(l GPKGLayer) SRID() (int) {return l.srid}

func (p *GPKGProvider) Layers() ([]mvt.LayerInfo, error) {
	fmt.Println("Attempting gpkg.Layers()")
	layerCount := len(p.layers)
	ls := make([]mvt.LayerInfo, layerCount)
	
	i := 0
	for _, layer := range p.layers {
		l := GPKGLayer{name: layer.name, srid: layer.srid}
		ls[i] = l
		i++
	}

	fmt.Println("Ok, returning mvt.LayerInfo array: ", ls)
	return ls, nil
}

func doScan(rows* sql.Rows, fid *int, geom *tegola.Geometry, gid *uint64, featureColValues []string,
			featureColNames []string) {
	switch len(featureColValues) {
		case 0:
			rows.Scan(&fid, &geom, &gid)
		case 1:
			rows.Scan(fid, geom, gid, &featureColValues[0])
		case 2:
			rows.Scan(fid, geom, gid, &featureColValues[0], &featureColValues[1])
		case 10:
			rows.Scan(fid, geom, gid, &featureColValues[0], &featureColValues[1],
				&featureColValues[2], &featureColValues[3], &featureColValues[4],
				&featureColValues[5], &featureColValues[6], &featureColValues[7],
				&featureColValues[8], &featureColValues[9])
	}

	for i := 0; i < len(featureColValues); i++ {
		if featureColValues[i] == "" {continue}

		fmt.Printf("(%v) %v: %v, ", i, featureColNames[i], featureColValues[i])
		fmt.Println()
	}
}

func getFeatures(layer *mvt.Layer, rows *sql.Rows, colnames []string) {
	var featureColValues []string
	featureColValues = make([]string, len(colnames) - 3)
	var fid int
	var geom tegola.Geometry
	var gid uint64
	
	for rows.Next() {
		doScan(rows, &fid, &geom, &gid, featureColValues, colnames[3:])
		
		// Add features to Layer
		layer.AddFeatures(mvt.Feature{
			ID:       &gid,
			Tags:     nil,
//			Tags:     gtags,
			Geometry: geom,
		})
	}
}

func (p *GPKGProvider) MVTLayer(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{}) (*mvt.Layer, error) {
	fmt.Println("Attempting MVTLayer()")
	filepath := p.config

	fmt.Println("Opening gpkg at: ", filepath)
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	// Get all feature rows for the layer requested.
	qtext := "SELECT * FROM " + layerName + ";"
	rows, err := db.Query(qtext)
	if err != nil {
		fmt.Println("Error during query: ", qtext, " - ", err)
		return nil, err
	}
	defer rows.Close()

	// Pick out column names & types
	var colnames []string
	colnames, _ = rows.Columns()
	fmt.Printf("Columns for %v: %v\n", layerName, colnames)
	ncol := len(colnames)
	fmt.Println("Column Count: ", ncol)
	var sqlColNames []string
	sqlColNames = make([]string, ncol)
	// Put the expected non-text columns at the beginning of the query in expected order
	sqlColNames[0] = "fid"
	sqlColNames[1] = "geom"
	sqlColNames[2] = "osm_id"
	
	nextFreeNameIdx := 3
	for _, colname := range colnames {
		if colname == "fid" {continue}
		if colname == "geom" {continue}
		if colname == "osm_id" {continue}
		sqlColNames[nextFreeNameIdx] = colname
		nextFreeNameIdx++
	}

	qtext_columns_ordered := "SELECT "
	for i, colname := range sqlColNames {
		qtext_columns_ordered += colname
		if i < ncol - 1 {
			qtext_columns_ordered += ","
		}
	}
	qtext_columns_ordered += " FROM " + layerName + ";"
	rows_columns_ordered, err := db.Query(qtext_columns_ordered)

	//	new mvt.Layer
	layer := new(mvt.Layer)
	layer.Name = layerName
	getFeatures(layer, rows_columns_ordered, sqlColNames)
	
//	var coltypes []reflect.Type
//	coltypes = make([]reflect.Type, ncol)
//	coltypes, _ = rows.DeclTypes()
//	fmt.Printf("Column types for %v: %v\n", layerName, coltypes)


	msg := "MVTLayer() implementation in progress"
	e := errors.New(msg)
	fmt.Println(msg)
	return nil, e
}


func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	m := dict.M(config)
	filepath, err := m.String(FilePath, nil)
	if err != nil {
		return nil, err
	}
	
	fmt.Println("Attempting sql.Open() w/ filepath: ", filepath)
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}

	p := GPKGProvider{config: filepath, layers: make(map[string]layer)}

	qtext := "SELECT * FROM gpkg_contents"
	rows, err := db.Query(qtext)
	if err != nil {
		fmt.Println("Error during query: ", qtext, " - ", err)
		return nil, err
	}
	defer rows.Close()

	var tablename string
	var srid int
	var ignore string
	for rows.Next() {
		rows.Scan(&tablename, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &srid)
		layerQuery := "SELECT * FROM " + tablename + ";"
		p.layers[tablename] = layer{name: tablename, sql: layerQuery, geomType: "", srid: srid}
		fmt.Println("gpkg_contents row: ", tablename, srid)
	}
	
	return &p, err
}

func init() {
	fmt.Println("Entering gpkg.go init()")
	provider.Register(Name, NewProvider)
}