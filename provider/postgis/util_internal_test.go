package postgis

import (
	"context"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/go-spatial/tegola/provider"
)

func TestReplaceTokens(t *testing.T) {
	type tcase struct {
		sql      string
		tile     provider.Tile
		expected string
		layer    Layer
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			sql, err := replaceTokens(tc.sql, &tc.layer, tc.tile, true)
			if err != nil {
				t.Errorf("unexpected error, Expected nil Got %v", err)
				return
			}

			if sql != tc.expected {
				t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expected, sql)
				return
			}
		}
	}

	tests := map[string]tcase{
		"replace BBOX": {
			sql:      "SELECT * FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-10175297.20532266,-156543.03392804,156543.03392804,10175297.20532266,3857)",
		},
		"replace BBOX with != in query": {
			sql:      "SELECT * FROM foo WHERE geom && !BBOX! AND bar != 42",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-10175297.20532266,-156543.03392804,156543.03392804,10175297.20532266,3857) AND bar != 42",
		},
		"replace BBOX and ZOOM 1": {
			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-10175297.20532266,-156543.03392804,156543.03392804,10175297.20532266,3857)",
		},
		"replace BBOX and ZOOM 2": {
			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=16 FROM foo WHERE geom && ST_MakeEnvelope(-13163688.81778845,4035254.04260249,-13163058.21230510,4035884.64808584,3857)",
		},
		"replace pixel_width/height and scale_denominator": {
			sql:      "SELECT id, !pixel_width! as width, !pixel_height! as height, !scale_denominator! as scale_denom FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(11, 1070, 676, 64, tegola.WebMercator),
			expected: "SELECT id, 76.43702829 as width, 76.43702829 as height, 272989.38673277 as scale_denom FROM foo WHERE geom && ST_MakeEnvelope(899816.69697309,6789748.34851564,919996.07244038,6809927.72398292,3857)",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestUppercaseTokens(t *testing.T) {
	type tcase struct {
		str      string
		expected string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			out := uppercaseTokens(tc.str)

			if out != tc.expected {
				t.Errorf("expected \n \t%v\n out \n \t%v", tc.expected, out)
				return
			}
		}
	}

	tests := map[string]tcase{
		"uppercase tokens": {
			str:      "this !lower! case !STrInG! should uppercase !TOKENS!",
			expected: "this !LOWER! case !STRING! should uppercase !TOKENS!",
		},
		"no tokens": {
			str:      "no token",
			expected: "no token",
		},
		"empty string": {
			str:      "",
			expected: "",
		},
		"unclosed token": {
			str:      "unclosed !token",
			expected: "unclosed !token",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestDecipherFields(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	ctx := t.Context()

	type tcase struct {
		sql              string
		expectedRowCount int
		expectedTags     map[string]any
	}

	uri := ttools.GetEnvDefault("PGURI", "postgres://postgres:postgres@localhost:5432/tegola")
	c := newDefaultConnector(dict.Dict{"uri": uri})

	pool, _, _, err := c.Connect(ctx)
	if err != nil {
		t.Fatalf("unable to connect: %s", err)
	}
	defer pool.Close()

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			rows, err := pool.Query(context.Background(), tc.sql)
			if err != nil {
				t.Errorf("Error performing query: %v", err)
				return
			}
			defer rows.Close()

			var rowCount int
			for rows.Next() {
				geoFieldname := "geom"
				idFieldname := "id"
				descriptions := rows.FieldDescriptions()

				vals, err := rows.Values()
				if err != nil {
					t.Errorf("unexpected error reading row Values: %v", err)
					return
				}

				_, _, tags, err := decipherFields(
					context.TODO(),
					geoFieldname,
					idFieldname,
					descriptions,
					vals,
				)
				if err != nil {
					t.Errorf("unexpected error running decipherFileds: %v", err)
					return
				}

				if len(tags) != len(tc.expectedTags) {
					t.Errorf(
						"got (%v): %#v, expected (%v): %#v",
						len(tags),
						tags,
						len(tc.expectedTags),
						tc.expectedTags,
					)
					return
				}

				for k, v := range tags {
					if tc.expectedTags[k] != v {
						t.Errorf(
							"missing or bad value for tag %v: %v (%T) != %v (%T)",
							k,
							v,
							v,
							tc.expectedTags[k],
							tc.expectedTags[k],
						)
						return
					}
				}

				rowCount++
			}
			if rows.Err() != nil {
				t.Errorf("unexpected err: %v", rows.Err())
				return
			}

			if rowCount != tc.expectedRowCount {
				t.Errorf("invalid row count. expected %v. got %v", tc.expectedRowCount, rowCount)
				return
			}
		}
	}

	tests := map[string]tcase{
		"tags with hstore": {
			sql:              "SELECT name, extra_text, extra_int, properties FROM test_tags_table WHERE id = 1;",
			expectedRowCount: 1,
			expectedTags: map[string]any{
				"name":        "Polygon A",
				"count":       "42",
				"enabled":     "true",
				"price":       "19.99",
				"description": "example polygon A",
				"extra_text":  "Additional info A",
				"extra_int":   int64(100),
			},
		},
		"tags with uuid": {
			sql:              "SELECT uuid FROM test_tags_table WHERE id = 1;",
			expectedRowCount: 1,
			expectedTags: map[string]any{
				"uuid": "550e8400-e29b-41d4-a716-446655440000",
			},
		},
		// NOTE: should they or should they not?
		"tags do not contain primary key": {
			sql:              "SELECT id FROM test_tags_table WHERE id = 2;",
			expectedRowCount: 1,
			expectedTags:     map[string]any{},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
