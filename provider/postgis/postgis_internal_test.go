package postgis

import (
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/terranodo/tegola/internal/ttools"
	"github.com/terranodo/tegola/geom"
)

// TESTENV is the environment variable that must be set to "yes" to run postgis tests.
const TESTENV = "RUN_POSTGIS_TESTS"

func GetTestPort(t *testing.T) int64{
	ttools.ShouldSkip(t,TESTENV)
	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 64)
	if err != nil {
		t.Skipf("err parsing PGPORT: %v", err)
	}
	return port
}

func TestLayerGeomType(t *testing.T) {
	port := GetTestPort(t)

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
		//	zoom token replacement
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
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE gid = !ZOOM! AND geom && !BBOX!",
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
			t.Errorf("[%v] NewProvider error, expected nil got %v", i, err)
			continue
		}

		p := provider.(Provider)
		layer := p.layers[tc.layerName]
		if err := p.layerGeomType(&layer); err != nil {
			t.Errorf("[%v] layerGeomType error, expected nil got %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tc.geom, layer.geomType) {
			t.Errorf("[%v] geom type, expected %v got %v", i, tc.geom, layer.geomType)
			continue
		}
	}
}
