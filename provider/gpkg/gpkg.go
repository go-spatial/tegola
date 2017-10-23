package gpkg

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/util/dict"

	log "github.com/sirupsen/logrus"

	"context"
//	"errors"

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
	// Currently just the path to the gpkg file.
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
	log.Debug("Attempting gpkg.Layers()")
	layerCount := len(p.layers)
	ls := make([]mvt.LayerInfo, layerCount)
	
	i := 0
	for _, layer := range p.layers {
		l := GPKGLayer{name: layer.name, srid: layer.srid}
		ls[i] = l
		i++
	}

	log.Debugf("Ok, returning mvt.LayerInfo array: %v", ls)
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
	
	fmt.Println("Columns: ", cols)
	for rows.Next() {
        err = rows.Scan(valPtrs...)
        if err != nil {
            fmt.Println(err)
            continue
        }
        var gid uint64
        var geom tegola.Geometry

		for i := 0; i < len(vals); i++ {
			if vals[i] == nil {
				fmt.Printf("%v is <nil>\n", cols[i])
			} else if o, ok := vals[i].(int); ok {
				fmt.Println("value is an int")
				fmt.Println(int(o))
			} else if o, ok := vals[i].(int64); ok {
				fmt.Printf("%v: (%v) is an int64\n", cols[i], o)
			} else if o, ok := vals[i].(string); ok {
				fmt.Println("value is a string")
				fmt.Println(string(o))
			} else if o, ok := vals[i].([]uint8); ok {
				fmt.Printf("%v: (%v) is a []uint8\n", cols[i], o)
				if cols[i] != "geom" {
					fmt.Println("As string: ", string(o))
				}
			} else {
				fmt.Printf("%v: (%v) - (%T)\n", cols[i], vals[i], vals[i])
//				fmt.Println("reflect.ValueOf: ", reflect.ValueOf(&vals[i]))
				
			}
			
			if cols[i] == "geom" {
				fmt.Println("Doing geom extraction...", vals[i])
				var h GeoPackageBinaryHeader
				h.Init(vals[i].([]byte))
				
				if h.SRSId() != 4326 {
					fmt.Println("SRID ", pLayer.srid, " != 4326, trying to convert...")			
					// We need to convert our points to Webmercator.
					g, err := basic.ToWebMercator(pLayer.srid, geom)
					if err != nil {
						return nil, fmt.Errorf("Was unable to transform geometry to webmercator from SRID (%v) for layer (%v)", pLayer.srid, layerName)
					} else {
						fmt.Println("ok")
					}
					geom = g.Geometry
				} else {
					geom = vals[i].([]byte)[h.Size():]
				}
			}
		}

		if geom == nil {
			fmt.Println("No geometry, skipping feature")
			fmt.Println("---")
			continue
		}
		fmt.Println("---")


		newLayer.AddFeatures(mvt.Feature{
			ID: &gid,
			Tags: make(map[string]interface{}),
			Geometry: geom,
		})
		
	}

//			fmt.Println(vals[i])
//		}
	fmt.Println()

	// "fid", "geom", "osm_id"

//	getFeatures(layer, rows_columns_ordered, sqlColNames)
	
//	var coltypes []reflect.Type
//	coltypes = make([]reflect.Type, ncol)
//	coltypes, _ = rows.DeclTypes()
//	fmt.Printf("Column types for %v: %v\n", layerName, coltypes)


	return newLayer, nil
}


func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	m := dict.M(config)
	filepath, err := m.String("config", nil)
	if err != nil {
		return nil, err
	}
	
	log.Debug("Attempting sql.Open() w/ filepath: ", filepath)
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
	
	logMsg := "gpkg_contents: "
	for rows.Next() {
		rows.Scan(&tablename, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &ignore, &srid)
		layerQuery := "SELECT * FROM " + tablename + ";"
		p.layers[tablename] = layer{name: tablename, sql: layerQuery, geomType: "", srid: srid}
		logMsgPart := "(" + tablename + "," + string(srid) + ")"
		logMsg += logMsgPart
	}
	log.Debug(logMsg)

	return &p, err
}


func init() {
	log.Debug("Entering gpkg.go init()")
	provider.Register(Name, NewProvider)
}
