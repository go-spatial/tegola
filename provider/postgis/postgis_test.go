package postgis_test

import (
	"os"
	"testing"

	"context"

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
		//	scalerank test
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
		//	decode numeric(x,x) types
		{
			config: map[string]interface{}{
				postgis.ConfigKeyHost:     "localhost",
				postgis.ConfigKeyPort:     int64(5432),
				postgis.ConfigKeyDB:       "tegola",
				postgis.ConfigKeyUser:     "postgres",
				postgis.ConfigKeyPassword: "",
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName:   "buildings",
						postgis.ConfigKeyGeomIDField: "osm_id",
						postgis.ConfigKeySQL:         "SELECT ST_AsBinary(geometry) AS geometry, osm_id, name, nullif(as_numeric(height),-1) AS height, type FROM osm_buildings_test WHERE geometry && !BBOX!",
					},
				},
			},
			tile: tegola.Tile{
				Z: 16,
				X: 11241,
				Y: 26168,
			},
			expectedFeatureCount: 101,
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

			l, err := p.MVTLayer(context.Background(), layerName, tc.tile, map[string]interface{}{})
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
