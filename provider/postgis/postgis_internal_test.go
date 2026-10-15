package postgis

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	vectorTile "github.com/alexeydott/geom/encoding/mvt/vector_tile"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/ttools"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/test/fixture"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
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
	ExpectedErr    error
}

func (cfg TCConfig) Config(mConfig map[string]any) dict.Dict {
	if cfg.BaseConfig != nil {
		mConfig = cfg.BaseConfig
	}
	return fixture.Config(mConfig, cfg.ConfigOverride, cfg.LayerConfig)
}

func TestMVTProviders(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		TCConfig
		layerNames []string
		err        string
		tile       provider.Tile
	}
	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config(DefaultEnvConfig)
			config[ConfigKeyName] = "provider_name"
			prvd, err := NewMVTTileProvider(config, nil)
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
			// Encoding size changes across PostGIS/GEOS releases. Validate
			// decoded content rather than pinning one encoder's byte count.
			var decoded vectorTile.Tile
			if err := proto.Unmarshal(mvtTile, &decoded); err != nil {
				t.Fatal(err)
			}
			if len(decoded.Layers) != len(tc.layerNames) {
				t.Fatalf("layers = %d, want %d", len(decoded.Layers), len(tc.layerNames))
			}
			for i, layer := range decoded.Layers {
				if layer.GetName() != tc.layerNames[i] || layer.GetExtent() != 4096 || layer.GetVersion() != 2 {
					t.Fatalf("unexpected layer metadata: %v", layer)
				}
				if len(layer.Features) == 0 {
					t.Fatal("empty MVT layer")
				}
				seen := map[uint64]bool{}
				for _, feature := range layer.Features {
					if feature.Id == nil || feature.GetId() == 0 || seen[feature.GetId()] {
						t.Fatalf("missing or duplicate feature ID: %v", feature.Id)
					}
					seen[feature.GetId()] = true
					if feature.GetType() != vectorTile.Tile_POLYGON {
						t.Fatalf("geometry type = %v, want polygon", feature.GetType())
					}
					if _, err := mvt.DecodeGeometry(feature.GetType(), feature.Geometry); err != nil {
						t.Fatalf("invalid MVT geometry: %v", err)
					}
					if len(feature.Tags)%2 != 0 {
						t.Fatal("unpaired feature tags")
					}
					hasScaleRank := false
					for j := 0; j < len(feature.Tags); j += 2 {
						key, value := feature.Tags[j], feature.Tags[j+1]
						if int(key) >= len(layer.Keys) || int(value) >= len(layer.Values) {
							t.Fatal("tag references invalid dictionary entry")
						}
						hasScaleRank = hasScaleRank || layer.Keys[key] == "scalerank"
					}
					if !hasScaleRank {
						t.Fatal("feature lost scalerank attribute")
					}
				}
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

func TestPGXOnNotice(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	ctx := t.Context()

	tc := &TCConfig{}
	cfg := tc.Config(DefaultEnvConfig)
	c := newDefaultConnector(cfg)

	_, pgxCfg, _, err := c.Connect(ctx)
	if err != nil {
		t.Fatal("connecting should not error")
	}

	if pgxCfg.ConnConfig.Tracer == nil {
		t.Fatal("tracer should not be nil on dbconfig")
	}

	var noticeBuffer bytes.Buffer

	// Set the OnNotice callback to write the notice messages into our buffer.
	pgxCfg.ConnConfig.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
		noticeBuffer.WriteString(n.Message)
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		t.Fatalf("connecting to pool: %s", err)
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
