package postgis

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/go-spatial/tegola/provider"
)

// TESTENV is the environment variable that must be set to "yes" to run postgis tests.
const TESTENV = "RUN_POSTGIS_TESTS"

var defaultEnvConfig map[string]interface{}

func getConfigFromEnv() map[string]interface{} {
	port, err := strconv.Atoi(ttools.GetEnvDefault("PGPORT", "5432"))
	if err != nil {
		// if port is anything but int, fallback to default
		port = 5432
	}

	return map[string]interface{}{
		ConfigKeyHost:        ttools.GetEnvDefault("PGHOST", "localhost"),
		ConfigKeyPort:        port,
		ConfigKeyDB:          ttools.GetEnvDefault("PGDATABASE", "tegola"),
		ConfigKeyUser:        ttools.GetEnvDefault("PGUSER", "postgres"),
		ConfigKeyPassword:    ttools.GetEnvDefault("PGPASSWORD", "postgres"),
		ConfigKeySSLMode:     ttools.GetEnvDefault("PGSSLMODE", "disable"),
		ConfigKeySSLKey:      ttools.GetEnvDefault("PGSSLKEY", ""),
		ConfigKeySSLCert:     ttools.GetEnvDefault("PGSSLCERT", ""),
		ConfigKeySSLRootCert: ttools.GetEnvDefault("PGSSLROOTCERT", ""),
	}
}

func init() {
	defaultEnvConfig = getConfigFromEnv()
}

type TCConfig struct {
	BaseConfig     map[string]interface{}
	ConfigOverride map[string]interface{}
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
			prvd, err := NewMVTTileProvider(config)
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
			mvtTile, err := prvd.MVTForLayers(context.Background(), tc.tile, layers)
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
				ConfigOverride: map[string]interface{}{
					ConfigKeyUser: ttools.GetEnvDefault("PGUSER_NO_ACCESS", "tegola_no_access"),
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
					ConfigKeyURI: fmt.Sprintf("postgres://%v:%v@%v:%v/%v",
						defaultEnvConfig["user"],
						defaultEnvConfig["password"],
						defaultEnvConfig["host"],
						defaultEnvConfig["port"],
						defaultEnvConfig["database"],
					),
					ConfigKeyHost: "",
					ConfigKeyPort: "",
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
		"valid default config": {
			expectedUri: "postgres://postgres:postgres@localhost:5432/tegola?pool_max_conn_idle_time=30m&pool_max_conn_lifetime=1h&pool_max_conns=100&sslmode=disable",
		},
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
		"invalid host": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyHost: 0,
				},
			},
			err: "config: value mapped to \"host\" is int not string",
		},
		"invalid port": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyPort: "fails",
				},
			},
			err: "config: value mapped to \"port\" is string not int",
		},
		"invalid db": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyDB: false,
				},
			},
			err: "config: value mapped to \"database\" is bool not string",
		},
		"invalid user": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyUser: false,
				},
			},
			err: "config: value mapped to \"user\" is bool not string",
		},
		"invalid password": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyPassword: false,
				},
			},
			err: "config: value mapped to \"password\" is bool not string",
		},
		"invalid maxcon": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyMaxConn: false,
				},
			},
			err: "config: value mapped to \"max_connections\" is bool not int",
		},
		"invalid conn idle time": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyMaxConnIdleTime: false,
				},
			},
			err: "config: value mapped to \"max_connection_idle_time\" is bool not string",
		},
		"invalid conn lifetime": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]interface{}{
					ConfigKeyMaxConnLifetime: false,
				},
			},
			err: "config: value mapped to \"max_connection_lifetime\" is bool not string",
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
			config := tc.Config()
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
