package gpkg

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util/dict"
	"context"
	"errors"
	
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

func (p *GPKGProvider) MVTLayer(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{}) (*mvt.Layer, error) {
	fmt.Println("MVTLayer() not implemented for gpkg provider")
	e := errors.New("MVTLayer() not implemented for gpkg provider")
	return nil, e
}

func (p *GPKGProvider) Layers() ([]mvt.LayerInfo, error) {
	fmt.Println("Attempting gpkg.Layers()")
	var ls []mvt.LayerInfo

	fmt.Println("Ok, returning mvt.LayerInfo array: ", ls)
	return ls, nil
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
	ls := map[string]string{}

	fmt.Println("db, ls: ", db, ls)

	p := GPKGProvider{}
	
	return &p, nil	
}

func init() {
	fmt.Println("Entering gpkg.go init()")
	provider.Register(Name, NewProvider)
}
