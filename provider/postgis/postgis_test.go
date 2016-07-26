package postgis_test

import (
	"log"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/provider/postgis"
)

func TestNewProvider(t *testing.T) {
	// The database connection string have the following JSON format:
	// { "host" : "host", port
	config := map[string]interface{}{
		postgis.ConfigKeyHost:     "localhost",
		postgis.ConfigKeyPort:     5432,
		postgis.ConfigKeyDB:       "gdey",
		postgis.ConfigKeyUser:     "gdey",
		postgis.ConfigKeyPassword: "",
		postgis.ConfigKeyLayers: map[string]map[string]interface{}{
			"buildings": map[string]interface{}{
				postgis.ConfigKeyTablename: "gis.zoning_base_3857",
			},
		},
	}
	p, err := postgis.NewProvider(config)
	if err != nil {
		t.Errorf("Failed to create a new provider. %v", err)
		return
	}

	tile := tegola.Tile{
		Z: 15,
		X: 12451,
		Y: 18527,
	}
	l, err := p.MVTLayer("buildings", tile, map[string]interface{}{"class": "park"})
	if err != nil {
		t.Errorf("Failed to create mvt layer. %v", err)
		return
	}
	log.Printf("Go to following layer %v\n", l)
}
