package postgis_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/ttools"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/postgis"

	"github.com/jackc/pgx/v5"
)

func TestNewTileProvider(t *testing.T) {
	ttools.ShouldSkip(t, postgis.TESTENV)

	fn := func(tc postgis.TCConfig) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config(postgis.DefaultEnvConfig)
			config[postgis.ConfigKeyName] = "provider_name"
			_, err := postgis.NewTileProvider(config, nil)
			if err != nil {
				if tc.ExpectedErr != nil && err.Error() == tc.ExpectedErr.Error() {
					return
				}
				t.Errorf("unable to create a new provider. expected err (%v) got err (%v)", tc.ExpectedErr, err)
				return
			}
			if tc.ExpectedErr != nil {
				t.Errorf("expected err (%v) got nil", tc.ExpectedErr)
			}
		}
	}

	tests := map[string]postgis.TCConfig{
		"1": {
			LayerConfig: []map[string]any{
				{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestSourceSRIDAutoDetect(t *testing.T) {
	ttools.ShouldSkip(t, postgis.TESTENV)

	uri := ttools.GetEnvDefault("PGURI", "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable")
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	const tbl4326 = "tegola_srid_detect_4326"
	const tblUnknown = "tegola_srid_detect_unknown"
	drop := func() {
		_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS "+tbl4326)
		_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS "+tblUnknown)
	}
	drop()
	t.Cleanup(drop)

	if _, err = conn.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE %v (gid serial primary key, geom geometry(Point, 4326));
		 INSERT INTO %v (geom) VALUES (ST_SetSRID(ST_MakePoint(10.5, 48.5), 4326));`,
		tbl4326, tbl4326)); err != nil {
		t.Fatalf("setup 4326 table: %v", err)
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE %v (gid serial primary key);`,
		tblUnknown)); err != nil {
		t.Fatalf("setup unknown table: %v", err)
	}

	type tcase struct {
		name         string
		providerCfg  map[string]any
		layerCfg     map[string]any
		expectedSRID uint64
		expectedErr  string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			layerCfg := map[string]any{
				postgis.ConfigKeyLayerName: "detect",
				postgis.ConfigKeyTablename: tbl4326,
			}
			for k, v := range tc.layerCfg {
				layerCfg[k] = v
			}
			config := map[string]any{
				postgis.ConfigKeyName:   "srid_detect_provider",
				postgis.ConfigKeyURI:    uri,
				postgis.ConfigKeyLayers: []map[string]any{layerCfg},
			}
			for k, v := range tc.providerCfg {
				config[k] = v
			}

			p, err := postgis.NewTileProvider(dict.Dict(config), nil)
			if tc.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.expectedErr)
				}
				if !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("expected error containing %q, got %q", tc.expectedErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTileProvider: %v", err)
			}

			lyrs, lerr := p.Layers()
			if lerr != nil {
				t.Fatalf("Layers: %v", lerr)
			}
			if len(lyrs) != 1 {
				t.Fatalf("layer count = %v, want 1", len(lyrs))
			}
			if got := lyrs[0].SRID(); got != tc.expectedSRID {
				t.Fatalf("layer srid = %v, want %v", got, tc.expectedSRID)
			}
		}
	}

	tests := []tcase{
		{
			name:         "native table 4326 without srid auto-detects",
			expectedSRID: uint64(4326),
		},
		{
			name:         "provider explicit srid wins over metadata",
			providerCfg:  map[string]any{postgis.ConfigKeySRID: 3857},
			expectedSRID: uint64(3857),
		},
		{
			name:         "layer srid wins over metadata",
			layerCfg:     map[string]any{postgis.ConfigKeySRID: 3857},
			expectedSRID: uint64(3857),
		},
		{
			name: "table without geometry column fails with controlled error",
			layerCfg: map[string]any{
				postgis.ConfigKeyLayerName: "detect",
				postgis.ConfigKeyTablename: tblUnknown,
			},
			expectedErr: "unable to auto-detect source SRID",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, fn(tc))
	}
}

func TestTileFeatures(t *testing.T) {
	ttools.ShouldSkip(t, postgis.TESTENV)

	type tcase struct {
		postgis.TCConfig
		tile                 provider.Tile
		expectedErr          error
		expectedFeatureCount int
		expectedTags         []string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config(postgis.DefaultEnvConfig)
			config[postgis.ConfigKeyName] = "provider_name"
			p, err := postgis.NewTileProvider(config, nil)
			if err != nil {
				if tc.expectedErr != nil && err.Error() == tc.expectedErr.Error() {
					return
				}
				t.Errorf("unexpected error; unable to create a new provider, expected: %v Got %v", tc.expectedErr, err)
				return
			}

			layerName := tc.LayerConfig[0][postgis.ConfigKeyLayerName].(string)

			var featureCount int
			err = p.TileFeatures(context.Background(), layerName, tc.tile, nil, func(f *provider.Feature) error {
				// only verify tags on first feature
				if featureCount == 0 {
					for _, tag := range tc.expectedTags {
						if _, ok := f.Tags[tag]; !ok {
							t.Errorf("expected tag %v in %v", tag, f.Tags)
							return nil
						}
					}
				}

				featureCount++

				return nil
			})
			if err != tc.expectedErr {
				t.Errorf("expected err (%v) got err (%v)", tc.expectedErr, err)
				return
			}

			if featureCount != tc.expectedFeatureCount {
				t.Errorf("feature count, expected %v got %v", tc.expectedFeatureCount, featureCount)
				return
			}
		}
	}

	tests := map[string]tcase{
		"tablename query": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 4032,
			expectedTags:         []string{"scalerank", "featurecla"},
		},
		"tablename query with fields": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					postgis.ConfigKeyFields:    []string{"scalerank"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 4032,
			expectedTags:         []string{"scalerank"},
		},
		"tablename query with fields and id as field": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName:   "land",
					postgis.ConfigKeyTablename:   "ne_10m_land_scale_rank",
					postgis.ConfigKeyGeomIDField: "gid",
					postgis.ConfigKeyFields:      []string{"gid", "scalerank"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 4032,
			expectedTags:         []string{"gid", "scalerank"},
		},
		"SQL sub-query": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "(SELECT gid, geom, featurecla FROM ne_10m_land_scale_rank LIMIT 100) AS sub",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query multi line": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL: ` (
					SELECT gid, geom, featurecla FROM ne_10m_land_scale_rank LIMIT 100
				) AS sub`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query and tablename": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "(SELECT gid, geom, featurecla FROM ne_10m_land_scale_rank LIMIT 100) AS sub",
					postgis.ConfigKeyTablename: "not_good_name",
				}},
			},
			tile: provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedErr: fmt.Errorf(
				"for %v layer (land) %v: only one of %v or %v can be specified",
				"postgis",
				0,
				postgis.ConfigKeyTablename,
				postgis.ConfigKeySQL,
			),
			expectedFeatureCount: 0,
		},
		"SQL sub-query space after prens": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "(  SELECT gid, geom, featurecla FROM ne_10m_land_scale_rank LIMIT 100) AS sub",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query space before prens": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "   (SELECT gid, geom, featurecla FROM ne_10m_land_scale_rank LIMIT 100) AS sub",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query with comments": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       " -- this is a comment\n-- accross multiple lines\n (SELECT gid, geom, scalerank FROM ne_10m_land_scale_rank LIMIT 100) AS sub -- another comment at the end",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"scalerank"},
		},
		"SQL sub-query with *": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "(SELECT * FROM ne_10m_land_scale_rank LIMIT 100) AS sub",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"scalerank", "featurecla"},
		},
		"SQL sub-query with * and fields": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "(SELECT * FROM ne_10m_land_scale_rank LIMIT 100) AS sub",
					postgis.ConfigKeyFields:    []string{"scalerank"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"scalerank"},
		},
		"SQL with !ZOOM!": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) AS geom FROM ne_10m_land_scale_rank WHERE scalerank=!ZOOM! AND geom && !BBOX!",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 98,
		},
		"SQL sub-query with token in SELECT": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeyGeomType:  "polygon", // required to disable SQL inspection
					postgis.ConfigKeySQL:       "(SELECT gid, geom, !ZOOM! * 2 AS doublezoom FROM ne_10m_land_scale_rank WHERE scalerank = !ZOOM! AND geom && !BBOX!) AS sub",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 98,
			expectedTags:         []string{"doublezoom"},
		},
		"SQL sub-query with fields": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       "(SELECT gid, geom, 1 AS a, '2' AS b, 3 AS c FROM ne_10m_land_scale_rank WHERE scalerank = !ZOOM! AND geom && !BBOX!) AS sub",
					postgis.ConfigKeyFields:    []string{"gid", "a", "b"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 98,
			expectedTags:         []string{"a", "b"},
			// expectedTags:         []string{"gid", "a", "b"}, TODO #383
		},
		"SQL with comments": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "land",
					postgis.ConfigKeySQL:       " -- this is a comment\n -- accross multiple lines \n \tSELECT gid, -- gid \nST_AsBinary(geom) AS geom -- geom \n FROM ne_10m_land_scale_rank WHERE scalerank=!ZOOM! AND geom && !BBOX! -- comment at the end",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 98,
		},
		// OSM relation IDs are negative; this fixture tests numeric attributes.
		// Map them to positive IDs under the provider's uint64 feature-ID contract.
		"decode numeric(x,x) types": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName:   "buildings",
					postgis.ConfigKeyGeomIDField: "osm_id",
					postgis.ConfigKeyGeomField:   "geometry",
					postgis.ConfigKeySQL:         "SELECT ST_AsBinary(geometry) AS geometry, abs(osm_id) AS osm_id, name, nullif(as_numeric(height),-1) AS height, type FROM osm_buildings_test WHERE geometry && !BBOX!",
				}},
			},
			tile:                 provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedFeatureCount: 101,
			expectedTags:         []string{"name", "type"}, // height can be null and therefore missing from the tags
		},
		"gracefully handle 3d point": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName:   "three_d_points",
					postgis.ConfigKeyGeomIDField: "id",
					postgis.ConfigKeyGeomField:   "geom",
					postgis.ConfigKeySQL:         "SELECT ST_AsBinary(geom) AS geom, id FROM three_d_test WHERE geom && !BBOX!",
				}},
			},
			tile:                 provider.NewTile(0, 0, 0, 64, tegola.WebMercator),
			expectedFeatureCount: 0,
		},
		"gracefully handle null geometry": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName:   "null_geom",
					postgis.ConfigKeyGeomIDField: "id",
					postgis.ConfigKeyGeomField:   "geometry",
					// this SQL is a workaround the normal !BBOX! WHERE clause. we're simulating a null geometry lookup in the table and don't want to filter by bounding box
					postgis.ConfigKeySQL: "SELECT id, ST_AsBinary(geometry) AS geometry, !BBOX! AS bbox FROM null_geom_test",
				}},
			},
			tile:                 provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedFeatureCount: 1,
			expectedTags:         []string{"bbox"},
		},
		"missing geom field name": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "missing_geom_field_name",
					postgis.ConfigKeyGeomField: "geom",
					// this SQL is a workaround the normal !BBOX! token check. We don't care about the bounding
					// box query, but rather simulating the missing geom column to trigger the error we're testing for.
					postgis.ConfigKeySQL: "SELECT ST_AsBinary(geom), !BBOX! AS bbox FROM three_d_test",
				}},
			},
			tile: provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedErr: postgis.ErrGeomFieldNotFound{
				GeomFieldName: "geom",
				LayerName:     "missing_geom_field_name",
			},
		},
		"empty geometry collection": {
			TCConfig: postgis.TCConfig{
				LayerConfig: []map[string]any{{
					postgis.ConfigKeyLayerName: "empty_geometry_collection",
					postgis.ConfigKeyGeomField: "geom",
					postgis.ConfigKeyGeomType:  "polygon", // bypass the geometry type sniff on init
					postgis.ConfigKeySQL:       "SELECT ST_AsBinary(ST_GeomFromText('GEOMETRYCOLLECTION EMPTY')) AS geom, !BBOX! AS bbox",
				}},
			},
			tile:                 provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedFeatureCount: 1,
			expectedTags:         []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
