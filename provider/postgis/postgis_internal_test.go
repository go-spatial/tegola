package postgis

import (
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/internal/dict"
	"github.com/go-spatial/tegola/internal/ttools"
)

// TESTENV is the environment variable that must be set to "yes" to run postgis tests.
const TESTENV = "RUN_POSTGIS_TESTS"

func GetTestPort(t *testing.T) int {
	ttools.ShouldSkip(t, TESTENV)
	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 32)
	if err != nil {
		t.Skipf("err parsing PGPORT: %v", err)
	}
	return int(port)
}

func TestLayerGeomType(t *testing.T) {
	port := GetTestPort(t)

	type tcase struct {
		config    dict.Dict
		layerName string
		geom      geom.Geometry
	}

	fn := func(t *testing.T, tc tcase) {
		provider, err := NewTileProvider(tc.config)
		if err != nil {
			t.Errorf("NewProvider unexpected error: %v", err)
			return
		}

		p := provider.(Provider)
		layer := p.layers[tc.layerName]
		if err := p.layerGeomType(&layer); err != nil {
			t.Errorf("layerGeomType unexpected error: %v", err)
			return
		}

		if !reflect.DeepEqual(tc.geom, layer.geomType) {
			t.Errorf("geom type, expected %v got %v", tc.geom, layer.geomType)
			return
		}
	}

	tests := map[string]tcase{
		"1": {
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
		"zoom token replacement": {
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

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
