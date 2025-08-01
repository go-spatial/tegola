package postgis

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/go-spatial/tegola/provider"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TESTENV is the environment variable that must be set to "yes" to run postgis tests.
const TESTENV = "RUN_POSTGIS_TESTS"

var DefaultEnvConfig map[string]any

var DefaultConfig map[string]any = map[string]any{
	ConfigKeyURI:         "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable",
	ConfigKeySSLMode:     "disable",
	ConfigKeySSLKey:      "",
	ConfigKeySSLCert:     "",
	ConfigKeySSLRootCert: "",
}

func getConfigFromEnv() map[string]any {
	return map[string]any{
		ConfigKeyURI: ttools.GetEnvDefault(
			"PGURI",
			"postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable",
		),
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
	BaseConfig     map[string]any
	ConfigOverride map[string]any
	LayerConfig    []map[string]any
}

func (cfg TCConfig) Config(mConfig map[string]any) dict.Dict {
	var config map[string]any
	if cfg.BaseConfig != nil {
		mConfig = cfg.BaseConfig
	}
	config = make(map[string]any, len(mConfig))
	for k, v := range mConfig {
		config[k] = v
	}

	// set the config overrides
	for k, v := range cfg.ConfigOverride {
		config[k] = v
	}

	if len(cfg.LayerConfig) > 0 {
		layerConfig, _ := config[ConfigKeyLayers].([]map[string]any)
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
				LayerConfig: []map[string]any{
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
				LayerConfig: []map[string]any{
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
				LayerConfig: []map[string]any{
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
				LayerConfig: []map[string]any{
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
				LayerConfig: []map[string]any{
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
				LayerConfig: []map[string]any{
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
				ConfigOverride: map[string]any{
					ConfigKeyURI: ttools.GetEnvDefault(
						"PGURI_NO_ACCESS",
						"postgres://tegola_no_access:postgres@localhost:5432/tegola",
					),
				},
				LayerConfig: []map[string]any{
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
				ConfigOverride: map[string]any{
					ConfigKeyURI: DefaultEnvConfig["uri"],
				},
				LayerConfig: []map[string]any{
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
				ConfigOverride: map[string]any{
					ConfigKeyURI: "postgres://postgres:postgres@localhost:5432/tegola",
				},
			},
			expectedUri: "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable",
		},
		"add sslmode of uri and dont overwrite with default": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: "postgres://postgres:postgres@localhost:5432/tegola?sslmode=prefer",
				},
			},
			expectedUri: "postgres://postgres:postgres@localhost:5432/tegola?sslmode=prefer",
		},
		"invalid uri": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: false,
				},
			},
			err: "config: value mapped to \"uri\" is bool not string",
		},
		"invalid uri scheme": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: "http://hi.de",
				},
			},
			err: "postgis: invalid uri (invalid connection scheme (http))",
		},
		"invalid uri missing user": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: "postgres://hi.de",
				},
			},
			err: "postgis: invalid uri (auth credentials missing)",
		},
		"invalid uri missing port": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: "postgres://postgres:postgres@localhost/bla",
				},
			},
			err: "postgis: splitting host port error: address localhost: missing port in address",
		},
		"invalid uri missing host": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: "postgres://postgres:postgres@:5432/bla",
				},
			},
			err: "postgis: invalid uri (address :5432: missing host in address)",
		},
		"invalid uri missing database": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
					ConfigKeyURI: "postgres://postgres:postgres@localhost:5432",
				},
			},
			err: "postgis: invalid uri (missing database)",
		},
		"invalid sslmode": {
			TCConfig: TCConfig{
				ConfigOverride: map[string]any{
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

func TestPGXOnNotice(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	tc := &TCConfig{}
	c := tc.Config(DefaultConfig)
	uri, _, err := BuildURI(c)
	if err != nil {
		t.Fatal("building the uri should not fail:", err)
	}

	dbconfig, err := BuildDBConfig(&DBConfigOptions{Uri: uri.String()})
	if err != nil {
		t.Fatal("building db config should not fail:", err)
	}
	if dbconfig.ConnConfig.Tracer == nil {
		t.Fatal("tracer should not be nil on dbconfig")
	}

	var noticeBuffer bytes.Buffer

	// Set the OnNotice callback to write the notice messages into our buffer.
	dbconfig.ConnConfig.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
		noticeBuffer.WriteString(n.Message)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), dbconfig)
	if err != nil {
		t.Fatal("creating a pool from config should not fail:", err)
	}
	defer pool.Close()

	r, err := pool.Query(context.Background(), "SELECT test_warning_log();")
	if err != nil {
		t.Fatal("querying a row should not fail:", err)
	}
	t.Cleanup(func() {
		r.Close()
	})

	for r.Next() {
		var result string
		if err := r.Scan(&result); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
	}

	if err := r.Err(); err != nil {
		t.Fatalf("error during row iteration: %v", err)
	}

	expectedMsg := "This is a test warning message"
	if !strings.Contains(noticeBuffer.String(), expectedMsg) {
		t.Errorf(
			"expected notice message %q not found in buffer, got: %s",
			expectedMsg,
			noticeBuffer.String(),
		)
	}
}
