package postgis

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/jackc/pgx"
)

func TestReplaceTokens(t *testing.T) {
	testcases := []struct {
		sql      string
		srid     uint64
		tile     *slippy.Tile
		expected string
	}{
		{
			sql:      "SELECT * FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,-156543.03390625,156543.03390624933,1.017529720390625e+07,3857)",
		},
		{

			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,-156543.03390625,156543.03390624933,1.017529720390625e+07,3857)",
		},
		{
			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=16 FROM foo WHERE geom && ST_MakeEnvelope(-1.3163688815956049e+07,4.0352540420407774e+06,-1.3163058210472783e+07,4.035884647524042e+06,3857)",
		},
	}

	for i, tc := range testcases {
		sql, err := replaceTokens(tc.sql, tc.srid, tc.tile)
		if err != nil {
			t.Errorf("[%v] unexpected error, Expected nil Got %v", i, err)
			continue
		}

		if sql != tc.expected {
			t.Errorf("[%v] incorrect sql,\n Expected \n \t%v\n Got \n \t%v", i, tc.expected, sql)
		}
	}
}

func TestDecipherFields(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)
	cc := pgx.ConnConfig{
		Host:     os.Getenv("PGHOST"),
		Port:     5432,
		Database: os.Getenv("PGDATABASE"),
		User:     os.Getenv("PGUSER"),
		Password: os.Getenv("PGPASSWORD"),
	}

	type TestCase struct {
		id           int32
		expectedTags map[string]interface{}
	}

	testCases := []TestCase{
		{
			id:           1,
			expectedTags: map[string]interface{}{"height": "9", "int8_test": int64(1000888)},
		},
		{
			id:           2,
			expectedTags: map[string]interface{}{"hello": "there", "good": "day", "int8_test": int64(8880001)},
		},
	}

	conn, err := pgx.Connect(cc)
	if err != nil {
		t.Errorf("Unable to connect to database: %v", err)
	}
	defer conn.Close()

	for _, tc := range testCases {
		sql := fmt.Sprintf("SELECT id, tags, int8_test FROM hstore_test WHERE id = %v;", tc.id)
		rows, err := conn.Query(sql)
		if err != nil {
			t.Errorf("Error performing query: %v", err)
		}
		defer rows.Close()

		i := 0
		for rows.Next() {
			geoFieldname := "geom"
			idFieldname := "id"
			descriptions := rows.FieldDescriptions()
			vals, err := rows.Values()
			if err != nil {
				t.Errorf("[%v] Problem collecting row values", i)
			}

			_, _, tags, err := decipherFields(context.TODO(), geoFieldname, idFieldname, descriptions, vals)
			if len(tags) != len(tc.expectedTags) {
				t.Errorf("[%v] Got %v tags, was expecting %v: %#v, %#v", i, len(tags), len(tc.expectedTags), tags, tc.expectedTags)
			}
			for k, v := range tags {
				if tc.expectedTags[k] != v {
					t.Errorf("[%v] Missing or bad value for tag %v: %v (%T) != %v (%T)", i, k, v, v, tc.expectedTags[k], tc.expectedTags[k])
				}
			}
			i++
		}
	}
}
