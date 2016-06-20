package server_test

import (
	"testing"

	"github.com/terranodo/tegola/server"
)

func TestTileNum2Deg(t *testing.T) {
	testcases := []struct {
		tile        server.Tile
		expectedLat float64
		expectedLng float64
	}{
		{
			tile: server.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			expectedLat: 66.51326044311185,
			expectedLng: -90,
		},
	}

	for i, test := range testcases {
		lat, lng := test.tile.Num2Deg()
		if lat != test.expectedLat {
			t.Errorf(
				"Failed test %v. Expected lat (%v), got (%v)",
				i,
				lat,
				lat,
			)

		}
		if lng != test.expectedLng {
			t.Errorf(
				"Failed test %v. Expected lng (%v), got (%v)",
				i,
				lng,
				lng,
			)
		}
	}
}
