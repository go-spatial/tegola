package postgis

import (
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/terranodo/tegola/geom"
)

func TestLayerGeomType(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TESTS") != "yes" {
		return
	}

	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 64)
	if err != nil {
		t.Fatalf("err parsing PGPORT: %v", err)
	}

	testcases := []struct {
		config    map[string]interface{}
		layerName string
		geom      geom.Geometry
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
						ConfigKeyLayerName: "land",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
	}

	for i, tc := range testcases {
		provider, err := NewTileProvider(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed on NewProvider. err: %v", i, err)
			continue
		}

		p := provider.(Provider)
		layer := p.layers[tc.layerName]
		if err := p.layerGeomType(&layer); err != nil {
			t.Errorf("testcase (%v) failed on layerGeomType. err: %v", i, err)
			continue
		}

		expectedGeomType := reflect.TypeOf(tc.geom)
		outputGeomType := reflect.TypeOf(layer.geomType)

		if expectedGeomType != outputGeomType {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, outputGeomType, expectedGeomType)
		}
	}
}
