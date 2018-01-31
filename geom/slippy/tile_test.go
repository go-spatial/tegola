package slippy_test

import (
	"testing"

	"github.com/gdey/tegola"
	"github.com/terranodo/tegola/geom/slippy"
)

func TestExtent(t *testing.T) {
	testcases := []struct {
		tile           *slippy.Tile
		expectedExtent [2][2]float64
	}{
		{
			tile: slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expectedExtent: [2][2]float64{
				[2]float64{-10018754.17, 10018754.17},
				[2]float64{0, 0},
			},
		},
		{
			tile: slippy.NewTile(16, 11436, 26461, 64, tegola.WebMercator),
			expectedExtent: [2][2]float64{
				[2]float64{-13044437.497219238996, 3856706.6986199953},
				[2]float64{-13043826.000993041, 3856095.202393799},
			},
		},
	}

	for i, tc := range testcases {
		extent, _ := tc.tile.Extent()

		if tc.expectedExtent != extent {
			t.Errorf("[%v] expected: %v got: %v", i, tc.expectedExtent, extent)
			continue
		}
	}
}

func TestBufferedExtent(t *testing.T) {
	testcases := []struct {
		tile           *slippy.Tile
		expectedExtent [2][2]float64
	}{
		{
			tile: slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expectedExtent: [2][2]float64{
				[2]float64{-1.017529720390625e+07, 1.017529720390625e+07},
				[2]float64{156543.03390624933, -156543.03390624933},
			},
		},
	}

	for i, tc := range testcases {
		bufferedExtent, _ := tc.tile.BufferedExtent()

		if tc.expectedExtent != bufferedExtent {
			t.Errorf("[%v] expected: %v got: %v", i, tc.expectedExtent, bufferedExtent)
			continue
		}
	}
}
