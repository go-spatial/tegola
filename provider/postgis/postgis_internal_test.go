package postgis

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/go-spatial/tegola/mvtprovider"
	"github.com/go-spatial/tegola/provider"
)

// TESTENV is the environment variable that must be set to "yes" to run postgis tests.
const TESTENV = "RUN_POSTGIS_TESTS"

var defaultEnvConfig map[string]interface{}

func GetTestPort() int {
	port, err := strconv.ParseInt(os.Getenv("PGPORT"), 10, 32)
	if err != nil {
		// Since this is happening at init time, have a sane default
		fmt.Fprintf(os.Stderr, "err parsing PGPORT: '%v' using default port: 5433", err)
		return 5433
	}
	return int(port)
}

func getConfigFromEnv() map[string]interface{} {
	port := GetTestPort()
	return map[string]interface{}{
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
}

func init() {
	defaultEnvConfig = getConfigFromEnv()
}

type TCConfig struct {
	BaseConfig     map[string]interface{}
	ConfigOverride map[string]string
	LayerConfig    []map[string]interface{}
}

func (cfg TCConfig) Config() dict.Dict {
	var config map[string]interface{}
	mConfig := defaultEnvConfig
	if cfg.BaseConfig != nil {
		mConfig = cfg.BaseConfig
	}
	config = make(map[string]interface{}, len(mConfig))
	for k, v := range mConfig {
		config[k] = v
	}

	// set the config overrides
	for k, v := range cfg.ConfigOverride {
		config[k] = v
	}

	if len(cfg.LayerConfig) > 0 {
		layerConfig, _ := config[ConfigKeyLayers].([]map[string]interface{})
		layerConfig = append(layerConfig, cfg.LayerConfig...)
		config[ConfigKeyLayers] = layerConfig
	}

	return dict.Dict(config)
}

func TestMVTProviders(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		TCConfig
		layerNames []string
		mvtTile    []byte
		err        string
		tile       provider.Tile
	}
	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config()
			provider, err := NewMVTTileProvider(config)
			// for now we will just check the length of the bytes.
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Logf("error %#v", err)
					t.Errorf("expected error with %v in NewMVTTileProvider, got: %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Errorf("NewMVTTileProvider unexpected error: %v", err)
				return
			}
			layers := make([]mvtprovider.Layer, len(tc.layerNames))

			for i := range tc.layerNames {
				layers[i] = mvtprovider.Layer{
					Name:    tc.layerNames[i],
					MVTName: tc.layerNames[i],
				}
			}
			mvtTile, err := provider.MVTForLayers(context.Background(), tc.tile, layers)
			if err != nil {
				t.Errorf("NewProvider unexpected error: %v", err)
				return
			}
			if len(tc.mvtTile) != len(mvtTile) {
				t.Errorf("tile byte length, exected %v got %v", len(tc.mvtTile), len(mvtTile))
			}
		}
	}
	tests := map[string]tcase{
		"1": tcase{
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						ConfigKeyGeomIDField: "gid",
						ConfigKeyGeomType:    "multipolygon",
						ConfigKeyGeomField:   "geom",
						ConfigKeyLayerName:   "land",
						ConfigKeySQL:         "SELECT ST_AsMVTGeom(geom,!BBOX!) as geom, gid, scalerank FROM ne_10m_land_scale_rank WHERE geom && !BBOX!",
						ConfigKeySRID:        4326,
					},
				},
			},
			layerNames: []string{"land"},
			mvtTile:    make([]byte, 174689),
			tile:       provider.NewTile(0, 0, 0, 16, 4326),
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}

func TestLayerGeomType(t *testing.T) {

	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		TCConfig
		layerName string
		geom      geom.Geometry
		err       string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config()
			provider, err := NewTileProvider(config)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Logf("error %#v", err)
					t.Errorf("expected error with %v in NewProvider, got: %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Errorf("NewProvider unexpected error: %v", err)
				return
			}

			p := provider.(*Provider)
			layer := p.layers[tc.layerName]

			if !reflect.DeepEqual(tc.geom, layer.geomType) {
				t.Errorf("geom type, expected %v got %v", tc.geom, layer.geomType)
				return
			}
		}
	}

	tests := map[string]tcase{
		"1": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
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
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						ConfigKeyLayerName: "land",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE gid = !ZOOM! AND geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"configured geometry_type": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						ConfigKeyLayerName: "land",
						ConfigKeyGeomType:  "multipolygon",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM invalid_table_to_check_query_table_was_not_inspected WHERE geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"configured geometry_type (case insensitive)": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						ConfigKeyLayerName: "land",
						ConfigKeyGeomType:  "MultiPolyGOn",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM invalid_table_to_check_query_table_was_not_inspected WHERE geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
		},
		"invalid configured geometry_type": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						ConfigKeyLayerName: "land",
						ConfigKeyGeomType:  "invalid",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM invalid_table_to_check_query_table_was_not_inspected WHERE geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
			err:       "unsupported geometry_type",
		},
		"role no access to table": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]string{
					ConfigKeyUser: os.Getenv("PGUSER_NO_ACCESS"),
				},
				LayerConfig: []map[string]interface{}{
					{
						ConfigKeyLayerName: "land",
						ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) FROM ne_10m_land_scale_rank WHERE geom && !BBOX!",
					},
				},
			},
			layerName: "land",
			geom:      geom.MultiPolygon{},
			err:       "error fetching geometry type for layer (land): ERROR: permission denied for table ne_10m_land_scale_rank (SQLSTATE 42501)",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
