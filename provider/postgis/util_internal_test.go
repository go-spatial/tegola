package postgis

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/internal/ttools"
)

func TestReplaceTokens(t *testing.T) {
	type tcase struct {
		sql      string
		srid     uint64
		tile     *slippy.Tile
		expected string
	}

	fn := func(t *testing.T, tc tcase) {
		sql, err := replaceTokens(tc.sql, tc.srid, tc.tile)
		if err != nil {
			t.Errorf("unexpected error, Expected nil Got %v", err)
			return
		}

		if sql != tc.expected {
			t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expected, sql)
			return
		}
	}

	tests := map[string]tcase{
		"replace BBOX": {
			sql:      "SELECT * FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(2, 1, 1),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,-156543.03390625,156543.03390625,1.017529720390625e+07,3857)",
		},
		"replace BBOX and ZOOM 1": {

			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(2, 1, 1),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,-156543.03390625,156543.03390625,1.017529720390625e+07,3857)",
		},
		"replace BBOX and ZOOM 2": {
			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(16, 11241, 26168),
			expected: "SELECT id, scalerank=16 FROM foo WHERE geom && ST_MakeEnvelope(-1.3163688815956049e+07,4.0352540420407765e+06,-1.3163058210472783e+07,4.035884647524042e+06,3857)",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestDecipherFields(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		sql              string
		expectedRowCount int
		expectedTags     map[string]interface{}
	}

	cc := pgx.ConnConfig{
		Host:     os.Getenv("PGHOST"),
		Port:     5432,
		Database: os.Getenv("PGDATABASE"),
		User:     os.Getenv("PGUSER"),
		Password: os.Getenv("PGPASSWORD"),
	}

	conn, err := pgx.Connect(cc)
	if err != nil {
		t.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close()

	fn := func(t *testing.T, tc tcase) {
		rows, err := conn.Query(tc.sql)
		defer rows.Close()
		if err != nil {
			t.Errorf("Error performing query: %v", err)
			return
		}

		var rowCount int
		for rows.Next() {
			geoFieldname := "geom"
			idFieldname := "id"
			descriptions := rows.FieldDescriptions()

			vals, err := rows.Values()
			if err != nil {
				t.Errorf("unexepcted error reading row Values: %v", err)
				return
			}

			_, _, tags, err := decipherFields(context.TODO(), geoFieldname, idFieldname, descriptions, vals)
			if err != nil {
				t.Errorf("unexepcted error running decipherFileds: %v", err)
				return
			}

			if len(tags) != len(tc.expectedTags) {
				t.Errorf("got %v tags, expecting %v: %#v, %#v", len(tags), len(tc.expectedTags), tags, tc.expectedTags)
				return
			}

			for k, v := range tags {
				if tc.expectedTags[k] != v {
					t.Errorf("missing or bad value for tag %v: %v (%T) != %v (%T)", k, v, v, tc.expectedTags[k], tc.expectedTags[k])
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

	tests := map[string]tcase{
		"hstore 1": {
			sql:              "SELECT id, tags, int8_test FROM hstore_test WHERE id = 1;",
			expectedRowCount: 1,
			expectedTags: map[string]interface{}{
				"height":    "9",
				"int8_test": int64(1000888),
			},
		},
		"hstore 2": {
			sql:              "SELECT id, tags, int8_test FROM hstore_test WHERE id = 2;",
			expectedRowCount: 1,
			expectedTags: map[string]interface{}{
				"hello":     "there",
				"good":      "day",
				"int8_test": int64(8880001),
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
