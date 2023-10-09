package cache

import (
	"testing"

	"github.com/go-spatial/geom/slippy"
)

func TestRangeFamilyAt(t *testing.T) {
	type coord struct {
		z, x, y uint
	}

	type tcase struct {
		tile     *slippy.Tile
		zoomAt   uint
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
			rangeFamilyAt(tc.tile, tc.zoomAt, func(tile *slippy.Tile) error {
				z, x, y := tile.ZXY()
				c := coord{z, x, y}

				coordList = append(coordList, c)

				return nil
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
			tile:   slippy.NewTile(0, 0, 0),
			zoomAt: 1,
			expected: []coord{
				{1, 0, 0},
				{1, 0, 1},
				{1, 1, 0},
				{1, 1, 1},
			},
		},
		"children 2": {
			tile:   slippy.NewTile(8, 3, 5),
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
			tile:   slippy.NewTile(1, 0, 0),
			zoomAt: 0,
			expected: []coord{
				{0, 0, 0},
			},
		},
		"parent 2": {
			tile:   slippy.NewTile(3, 3, 5),
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
