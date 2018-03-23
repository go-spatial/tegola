package slippy_test

import (
	"testing"
	"fmt"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/geom/slippy"
)

func TestExtent(t *testing.T) {
	testcases := []struct {
		tile           *slippy.Tile
		expectedExtent [2][2]float64
	}{
		{
			tile: slippy.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expectedExtent: [2][2]float64{
				{-10018754.17, 10018754.17},
				{0, 0},
			},
		},
		{
			tile: slippy.NewTile(16, 11436, 26461, 64, tegola.WebMercator),
			expectedExtent: [2][2]float64{
				{-13044437.497219238996, 3856706.6986199953},
				{-13043826.000993041, 3856095.202393799},
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
				{-1.017529720390625e+07, 1.017529720390625e+07},
				{156543.03390624933, -156543.03390624933},
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

func TestRangeFamilyAt(t *testing.T) {
	type coord struct {
		z, x, y uint
	}

	testcases := map[string]struct {
		tile     *slippy.Tile
		zoomAt   uint
		expected []coord
	}{
		"children 1": {
			tile:   slippy.NewTile(0, 0, 0, 0, tegola.WebMercator),
			zoomAt: 1,
			expected: []coord{
				{1, 0, 0},
				{1, 0, 1},
				{1, 1, 0},
				{1, 1, 1},
			},
		},
		"children 2": {
			tile:   slippy.NewTile(8, 3, 5, 0, tegola.WebMercator),
			zoomAt: 10,
			expected: []coord{
				{10, 12, 20},
				{10, 12, 21},
				{10, 12, 22},
				{10, 12, 23},
				//
				{10, 13, 20},
				{10, 13, 21},
				{10, 13, 22},
				{10, 13, 23},
				//
				{10, 14, 20},
				{10, 14, 21},
				{10, 14, 22},
				{10, 14, 23},
				//
				{10, 15, 20},
				{10, 15, 21},
				{10, 15, 22},
				{10, 15, 23},
			},
		},
		"parent 1": {
			tile: slippy.NewTile(1, 0, 0, 0, tegola.WebMercator),
			zoomAt: 0,
			expected: []coord{
				{0, 0, 0},
			},
		},
		"parent 2": {
			tile: slippy.NewTile(3, 3, 5, 0, tegola.WebMercator),
			zoomAt: 1,
			expected: []coord{
				{1, 0, 1},
			},
		},
	}

	isIn := func(arr []coord, c coord) bool {
		for _, v := range arr {
			if v == c {
				return true
			}
		}

		return false
	}

	for k, tc := range testcases {
		coordList := make([]coord, 0, len(tc.expected))
		tc.tile.RangeFamilyAt(tc.zoomAt, func(tile *slippy.Tile) error {
			z, x, y := tile.ZXY()
			c := coord{z, x, y}

			coordList = append(coordList, c)

			return nil
		})

		if len(coordList) != len(tc.expected) {
			t.Fatalf("[%v] expected coordinate list of length %d, got %d", k, len(tc.expected), len(coordList))
		}

		for _, v := range tc.expected {
			if !isIn(coordList, v) {
				t.Fatalf("[%v] expected coordinate %v missing from list %v", k, v, coordList)
			}
		}
	}
}

func TestRangeFamilyAtIterStop(t *testing.T) {
	testcases := map[string]struct {
		tile     *slippy.Tile
	}{
		"end iter": {
			tile:   slippy.NewTile(3, 3, 5, 0, tegola.WebMercator),
		},
	}

	for k, tc := range testcases {
		stopIter := fmt.Errorf("stop iter")

		err := tc.tile.RangeFamilyAt(1, func(tile *slippy.Tile) error {
			return stopIter
		})

		if err != stopIter {
			t.Fatalf("[%v] errors should reference same address", k)
		}
	}
}

func TestNewTileLatLon(t *testing.T) {
	testcases := map[string]struct{
		tile *slippy.Tile
	}{
		"0": {
			tile: slippy.NewTile(0, 0, 0, 0, tegola.WebMercator),
		},
		"1": {
			tile: slippy.NewTile(1, 1, 1, 0, tegola.WebMercator),
		},
		"2": {
			tile: slippy.NewTile(20, 12231, 1235770, 0, tegola.WebMercator),
		},
	}

	for k, tc := range testcases {
		extDeg := tc.tile.ExtentDegrees()

		z, x, y := tc.tile.ZXY()
		lat := (extDeg[0][0] + extDeg[1][0]) / 2
		lon := (extDeg[0][1] + extDeg[1][1]) / 2

		testTile := slippy.NewTileLatLon(z, lat, lon, 0, tegola.WebMercator)
		zt, xt, yt := testTile.ZXY()

		if !(zt == z && xt == x && yt == y) {
			t.Fatalf("[%v] expected zxy (%d, %d, %d) got (%d, %d, %d)", k, z, x, y, zt, xt, yt)
		}

	}
}