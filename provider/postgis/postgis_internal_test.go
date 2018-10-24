package postgis

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/dict"
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
		config         map[string]interface{}
		configOverride map[string]string
		layerConfig    map[string]interface{}
		layerName      string
		geom           geom.Geometry
		err            string
	}

	defaultConfig := map[string]interface{}{
		ConfigKeyHost:        os.Getenv("PGHOST"),
		ConfigKeyPort:        port,
		ConfigKeyDB:          os.Getenv("PGDATABASE"),
		ConfigKeyUser:        os.Getenv("PGUSER"),
		ConfigKeyPassword:    os.Getenv("PGPASSWORD"),
		ConfigKeySSLMode:     os.Getenv("PGSSLMODE"),
		ConfigKeySSLKey:      os.Getenv("PGSSLKEY"),
		ConfigKeySSLCert:     os.Getenv("PGSSLCERT"),
		ConfigKeySSLRootCert: os.Getenv("PGSSLROOTCERT"),
	}

	fn := func(t *testing.T, tc tcase) {
		// check if we have env vars to override
		if len(tc.configOverride) > 0 {
			conf := map[string]interface{}{}
			// copy the original config
			for k, v := range tc.config {
				conf[k] = v
			}

			// set the config overrides
			for k, v := range tc.configOverride {
				conf[k] = v
			}

			// override the test's config with our new one
			tc.config = conf
		}

		tc.config[ConfigKeyLayers] = []map[string]interface{}{tc.layerConfig}

		provider, err := NewTileProvider(dict.Dict(tc.config))
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("expected error with %q in NewProvider, got: %v", tc.err, err)
			}
			return
		}
		if err != nil {
			t.Errorf("NewProvider unexpected error: %v", err)
			return
		}

		p := provider.(Provider)
		layer := p.layers[tc.layerName]

		if !reflect.DeepEqual(tc.geom, layer.geomType) {
			t.Errorf("geom type, expected %v got %v", tc.geom, layer.geomType)
			return
		}
	}

	tests := map[string]tcase{
		"1": {
			config: defaultConfig,
			layerConfig: map[string]interface{}{
				ConfigKeyLayerName: "land",
				ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE geom && !BBOX!",
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"zoom token replacement": {
			config: defaultConfig,
			layerConfig: map[string]interface{}{
				ConfigKeyLayerName: "land",
				ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE gid = !ZOOM! AND geom && !BBOX!",
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"configured geometry_type": {
			config: defaultConfig,
			layerConfig: map[string]interface{}{
				ConfigKeyLayerName: "land",
				ConfigKeyGeomType:  "multipolygon",
				ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM invalid_table_to_check_query_table_was_not_inspected WHERE geom && !BBOX!",
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"configured geometry_type (case insensitive)": {
			config: defaultConfig,
			layerConfig: map[string]interface{}{
				ConfigKeyLayerName: "land",
				ConfigKeyGeomType:  "MultiPolyGOn",
				ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM invalid_table_to_check_query_table_was_not_inspected WHERE geom && !BBOX!",
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"invalid configured geometry_type": {
			config: defaultConfig,
			layerConfig: map[string]interface{}{
				ConfigKeyLayerName: "land",
				ConfigKeyGeomType:  "invalid",
				ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM invalid_table_to_check_query_table_was_not_inspected WHERE geom && !BBOX!",
			},
			layerName: "land",
			err:       "unsupported geometry_type",
			geom:      geom.MultiPolygon{},
		},
		"role no access to table": {
			config: defaultConfig,
			configOverride: map[string]string{
				ConfigKeyUser: os.Getenv("PGUSER_NO_ACCESS"),
			},
			err: "error fetching geometry type for layer (land): ERROR: permission denied for relation ne_10m_land_scale_rank (SQLSTATE 42501)",
			layerConfig: map[string]interface{}{
				ConfigKeyLayerName: "land",
				ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE geom && !BBOX!",
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
