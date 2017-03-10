package postgis_test

import (
	"log"
	"os"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/provider/postgis"
)

func TestNewProvider(t *testing.T) {
	// The database connection string have the following JSON format:
	// { "host" : "host", port
	if os.Getenv("RUN_POSTGRESS_TEST") == "" {
		return
	}

	config := map[string]interface{}{
		postgis.ConfigKeyHost:     "localhost",
		postgis.ConfigKeyPort:     int64(5432),
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

func TestReplaceTokens(t *testing.T) {

	testcases := []struct {
		layer    postgis.Layer
		tile     tegola.Tile
		expected string
	}{
		{
			layer: postgis.Layer{
				SQL:  "SELECT * FROM foo WHERE geom && !BBOX!",
				SRID: tegola.WebMercator,
			},
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.001875417e+07,1.001875417e+07,0,0,3857)",
		},
		{
			layer: postgis.Layer{
				SQL:  "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
				SRID: tegola.WebMercator,
			},
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.001875417e+07,1.001875417e+07,0,0,3857)",
		},
	}

	for i, tc := range testcases {
		sql, err := postgis.ReplaceTokens(&tc.layer, tc.tile)
		if err != nil {
			t.Errorf("Failed test %v. err: %v", i, err)
		}

		if sql != tc.expected {
			t.Errorf("Failed test %v. Expected (%v), got (%v)", i, tc.expected, sql)
		}
	}
}
