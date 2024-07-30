package cache

import (
	"testing"

	"github.com/go-spatial/geom/slippy"
)

func TestRangeFamilyAt(t *testing.T) {
	type coord struct {
		z    slippy.Zoom
		x, y uint
	}

	type tcase struct {
		tile     slippy.Tile
		zoomAt   slippy.Zoom
		expected []coord
	}

	isIn := func(arr []coord, c coord) bool {
		for _, v := range arr {
			if v == c {
				return true
			}
		}

		return false
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			coordList := make([]coord, 0, len(tc.expected))
			//for tile := range tc.tile.FamilyAt(tc.zoomAt)

			slippy.RangeFamilyAt(tc.tile, tc.zoomAt, func(tile slippy.Tile) bool {
				z, x, y := tile.ZXY()
				c := coord{z, x, y}

				coordList = append(coordList, c)

				return true
			})

			if len(coordList) != len(tc.expected) {
				t.Fatalf("coordinate list length, expected %d, got %d: %v \n\n %v", len(tc.expected), len(coordList), tc.expected, coordList)
			}

			for _, v := range tc.expected {
				if !isIn(coordList, v) {
					t.Logf("coordinates: %v", coordList)
					t.Fatalf("coordinate exists, expected %v,  got missing", v)
				}
			}

		}
	}

	testcases := map[string]tcase{
		"children 1": {
			tile:   slippy.Tile{},
			zoomAt: 1,
			expected: []coord{
				{1, 0, 0},
				{1, 0, 1},
				{1, 1, 0},
				{1, 1, 1},
			},
		},
		"children 2": {
			tile:   slippy.Tile{Z: 8, X: 3, Y: 5},
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
			tile:   slippy.Tile{Z: 1},
			zoomAt: 0,
			expected: []coord{
				{0, 0, 0},
			},
		},
		"parent 2": {
			tile:   slippy.Tile{Z: 3, X: 3, Y: 5},
			zoomAt: 1,
			expected: []coord{
				{1, 0, 1},
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, fn(tc))
	}
}
