package postgis

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/go-spatial/tegola/provider"
)

// TESTENV is the environment variable that must be set to "yes" to run postgis tests.
const TESTENV = "RUN_POSTGIS_TESTS"

var DefaultEnvConfig map[string]interface{}

var DefaultConfig map[string]interface{} = map[string]interface{}{
	ConfigKeyURI:         "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable",
	ConfigKeySSLMode:     "disable",
	ConfigKeySSLKey:      "",
	ConfigKeySSLCert:     "",
	ConfigKeySSLRootCert: "",
}

func getConfigFromEnv() map[string]interface{} {
	return map[string]interface{}{
		ConfigKeyURI:         ttools.GetEnvDefault("PGURI", "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable"),
		ConfigKeySSLMode:     ttools.GetEnvDefault("PGSSLMODE", "disable"),
		ConfigKeySSLKey:      ttools.GetEnvDefault("PGSSLKEY", ""),
		ConfigKeySSLCert:     ttools.GetEnvDefault("PGSSLCERT", ""),
		ConfigKeySSLRootCert: ttools.GetEnvDefault("PGSSLROOTCERT", ""),
	}
}

func init() {
	DefaultEnvConfig = getConfigFromEnv()
}

type TCConfig struct {
	BaseConfig     map[string]interface{}
	ConfigOverride map[string]interface{}
	LayerConfig    []map[string]interface{}
}

func (cfg TCConfig) Config(mConfig map[string]interface{}) dict.Dict {
	var config map[string]interface{}
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
			config := tc.Config(DefaultEnvConfig)
			config[ConfigKeyName] = "provider_name"
			prvd, err := NewMVTTileProvider(config, nil)
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
			layers := make([]provider.Layer, len(tc.layerNames))

			for i := range tc.layerNames {
				layers[i] = provider.Layer{
					Name:    tc.layerNames[i],
					MVTName: tc.layerNames[i],
				}
			}
			mvtTile, err := prvd.MVTForLayers(context.Background(), tc.tile, nil, layers)
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
		"1": {
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
			config := tc.Config(DefaultEnvConfig)
			config[ConfigKeyName] = "provider_name"
			provider, err := NewTileProvider(config, nil)
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
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: ttools.GetEnvDefault("PGURI_NO_ACCESS", "postgres://tegola_no_access:postgres@localhost:5432/tegola"),
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
		"configure from postgreql URI": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: DefaultEnvConfig["uri"],
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
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestBuildUri(t *testing.T) {

	type tcase struct {
		TCConfig
		expectedUri string
		err         string
	}

	tests := map[string]tcase{
		"add sslmode to uri if missing": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "postgres://postgres:postgres@localhost:5432/tegola",
				},
			},
			expectedUri: "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable",
		},
		"add sslmode of uri and dont overwrite with default": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "postgres://postgres:postgres@localhost:5432/tegola?sslmode=prefer",
				},
			},
			expectedUri: "postgres://postgres:postgres@localhost:5432/tegola?sslmode=prefer",
		},
		"invalid uri": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: false,
				},
			},
			err: "config: value mapped to \"uri\" is bool not string",
		},
		"invalid uri scheme": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "http://hi.de",
				},
			},
			err: "postgis: invalid uri (invalid connection scheme (http))",
		},
		"invalid uri missing user": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "postgres://hi.de",
				},
			},
			err: "postgis: invalid uri (auth credentials missing)",
		},
		"invalid uri missing port": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "postgres://postgres:postgres@localhost/bla",
				},
			},
			err: "postgis: splitting host port error: address localhost: missing port in address",
		},
		"invalid uri missing host": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "postgres://postgres:postgres@:5432/bla",
				},
			},
			err: "postgis: invalid uri (address :5432: missing host in address)",
		},
		"invalid uri missing database": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyURI: "postgres://postgres:postgres@localhost:5432",
				},
			},
			err: "postgis: invalid uri (missing database)",
		},
		"invalid sslmode": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeySSLMode: false,
				},
			},
			err: "config: value mapped to \"ssl_mode\" is bool not string",
		},
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config(DefaultConfig)
			uri, _, err := BuildURI(config)

			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Logf("error %#v", err)
					t.Errorf("expected error with %v in BuildURI, got: %v", tc.err, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if uri.String() != tc.expectedUri {
				t.Errorf("expected: %v, got: %v", tc.expectedUri, uri)
			}
		}
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
