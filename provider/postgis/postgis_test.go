package postgis_test

import (
	"os"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/provider/postgis"
)

func TestNewProvider(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TEST") != "yes" {
		return
	}

	testcases := []struct {
		config map[string]interface{}
	}{
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     "localhost",
				postgis.ConfigKeyPort:     int64(5432),
				postgis.ConfigKeyDB:       "tegola",
				postgis.ConfigKeyUser:     "postgres",
				postgis.ConfigKeyPassword: "",
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		_, err := postgis.NewProvider(tc.config)
		if err != nil {
			t.Errorf("Failed test %v. Unable to create a new provider. err: %v", i, err)
			return
		}
	}
}

func TestMVTLayer(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TEST") != "yes" {
		return
	}

	testcases := []struct {
		config               map[string]interface{}
		tile                 tegola.Tile
		expectedFeatureCount int
	}{
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     "localhost",
				postgis.ConfigKeyPort:     int64(5432),
				postgis.ConfigKeyDB:       "tegola",
				postgis.ConfigKeyUser:     "postgres",
				postgis.ConfigKeyPassword: "",
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					},
				},
			},
			tile: tegola.Tile{
				Z: 1,
				X: 1,
				Y: 1,
			},
			expectedFeatureCount: 614,
		},
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     "localhost",
				postgis.ConfigKeyPort:     int64(5432),
				postgis.ConfigKeyDB:       "tegola",
				postgis.ConfigKeyUser:     "postgres",
				postgis.ConfigKeyPassword: "",
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE scalerank=!ZOOM! AND geom && !BBOX!",
					},
				},
			},
			tile: tegola.Tile{
				Z: 1,
				X: 1,
				Y: 1,
			},
			expectedFeatureCount: 23,
		},
	}

	for i, tc := range testcases {
		p, err := postgis.NewProvider(tc.config)
		if err != nil {
			t.Errorf("Failed test %v. Unable to create a new provider. err: %v", i, err)
			return
		}

		//	iterate our configured layers
		for _, tcLayer := range tc.config[postgis.ConfigKeyLayers].([]map[string]interface{}) {
			layerName := tcLayer[postgis.ConfigKeyLayerName].(string)

			l, err := p.MVTLayer(layerName, tc.tile, map[string]interface{}{})
			if err != nil {
				t.Errorf("Failed to create mvt layer. %v", err)
				return
			}

			if len(l.Features()) != tc.expectedFeatureCount {
				t.Errorf("Failed test %v. Expected feature count (%v), got (%v)", i, tc.expectedFeatureCount, len(l.Features()))
				return
			}
		}
	}
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
			return
		}

		if sql != tc.expected {
			t.Errorf("Failed test %v. Expected (%v), got (%v)", i, tc.expected, sql)
			return
		}
	}
}
