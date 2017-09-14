package postgis

import (
	"os"
	"reflect"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

func TestLayerGeomType(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TEST") != "yes" {
		return
	}

	testcases := []struct {
		config    map[string]interface{}
		layerName string
		geom      tegola.Geometry
	}{
		{
			config: map[string]interface{}{
				ConfigKeyHost:     "localhost",
				ConfigKeyPort:     int64(5432),
				ConfigKeyDB:       "tegola",
				ConfigKeyUser:     "postgres",
				ConfigKeyPassword: "",
				ConfigKeyLayers: []map[string]interface{}{
					{
						ConfigKeyLayerName: "land",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      basic.MultiPolygon{},
		},
	}

	for i, tc := range testcases {
		provider, err := NewProvider(tc.config)
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
