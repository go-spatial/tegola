package postgis

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom/slippy"
	"github.com/terranodo/tegola/provider"
)

func TestForEachFeature(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TESTS") != "yes" {
		return
	}

	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 64)
	if err != nil {
		t.Fatalf("err parsing PGPORT: %v", err)
	}

	testcases := []struct {
		config       map[string]interface{}
		tile         *slippy.Tile
		expectedTags map[string]interface{}
	}{
		{
			config: map[string]interface{}{
				ConfigKeyHost:     os.Getenv("PGHOST"),
				ConfigKeyPort:     port,
				ConfigKeyDB:       os.Getenv("PGDATABASE"),
				ConfigKeyUser:     os.Getenv("PGUSER"),
				ConfigKeyPassword: os.Getenv("PGPASSWORD"),
				ConfigKeyLayers: []map[string]interface{}{
					{
						ConfigKeyLayerName:   "buildings",
						ConfigKeyGeomIDField: "id",
						ConfigKeyGeomField:   "geom",
						ConfigKeySQL:         "SELECT id, height, ST_AsBinary(geom) AS geom FROM hstore_test WHERE geom && !BBOX!",
					},
				},
			},
			tile: slippy.NewTile(1, 1, 1, 0, tegola.WebMercator),
			expectedTags: map[string]interface{}{
				"height": "10",
			},
		},
	}

	for i, tc := range testcases {
		tc := tc

		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			tileProvider, err := NewTileProvider(tc.config)
			if err != nil {
				t.Errorf(" NewTileProvider error, expected nil got %v", err)
				return
			}

			//	iterate our configured layers
			for _, tcLayer := range tc.config[ConfigKeyLayers].([]map[string]interface{}) {
				layerName := tcLayer[ConfigKeyLayerName].(string)
				tileProvider := tileProvider
				t.Run(layerName, func(t *testing.T) {
					t.Parallel()
					if err := tileProvider.TileFeatures(
						context.Background(),
						layerName,
						tc.tile,
						func(f *provider.Feature) error {
							if !reflect.DeepEqual(tc.expectedTags, f.Tags) {
								t.Errorf("[%v] tags failed, expected (%+v) got (%+v)", i, tc.expectedTags, f.Tags)
							}
							return nil
						},
					); err != nil {
						t.Errorf("[%v] err failed. expected nil got %v", i, err)
					}
				})
			}
		})
	}
}
