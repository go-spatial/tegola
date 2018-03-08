package postgis

import (
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/geom/slippy"
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
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.018e+07,-1.565e+05,1.565e+05,1.018e+07,3857)",
		},
		{

			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			srid:     tegola.WebMercator,
			tile:     slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.018e+07,-1.565e+05,1.565e+05,1.018e+07,3857)",
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
