package postgis

import (
	"testing"

	"github.com/terranodo/tegola"
)

func TestReplaceTokens(t *testing.T) {
	testcases := []struct {
		layer    Layer
		tile     *tegola.Tile
		expected string
	}{
		{
			layer: Layer{
				sql:  "SELECT * FROM foo WHERE geom && !BBOX!",
				srid: tegola.WebMercator,
			},
			tile:     tegola.NewTile(2, 1, 1),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,1.017529720390625e+07,156543.03390624933,-156543.03390624933,3857)",
		},
		{
			layer: Layer{
				sql:  "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
				srid: tegola.WebMercator,
			},
			tile:     tegola.NewTile(2, 1, 1),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.017529720390625e+07,1.017529720390625e+07,156543.03390624933,-156543.03390624933,3857)",
		},
	}

	for i, tc := range testcases {
		sql, err := replaceTokens(&tc.layer, tc.tile)
		if err != nil {
			t.Errorf("[%v] unexpected error, Expected nil Got %v", i, err)
			// Skip to next test
			continue
		}

		if sql != tc.expected {
			t.Errorf("[%v] incorrect sql, Expected (%v) Got (%v)", i, tc.expected, sql)
		}
	}
}
