package postgis

import (
	"testing"

	"github.com/terranodo/tegola"
)

func TestReplaceTokens(t *testing.T) {
	testcases := []struct {
		layer    layer
		tile     tegola.Tile
		expected string
	}{
		{
			layer: layer{
				SQL:  "SELECT * FROM foo WHERE geom && !BBOX!",
				SRID: tegola.WebMercator,
			},
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-1.001875417e+07,1.001875417e+07,0,0,3857)",
		},
		{
			layer: layer{
				SQL:  "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
				SRID: tegola.WebMercator,
			},
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-1.001875417e+07,1.001875417e+07,0,0,3857)",
		},
	}

	for i, tc := range testcases {
		sql, err := replaceTokens(&tc.layer, tc.tile)
		if err != nil {
			t.Errorf("Failed test %v. err: %v", i, err)
			return
		}

		if sql != tc.expected {
			t.Errorf("Failed test %v. Expected (%v), got (%v)", i, tc.expected, sql)
			return
		}
	}
}
