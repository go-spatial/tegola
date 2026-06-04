package tegola_test

import (
	"testing"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
)

func TestWorldCRS84QuadExtent(t *testing.T) {
	tests := map[string]struct {
		tile     slippy.Tile
		expected [4]float64
	}{
		"z0 west": {
			tile:     slippy.Tile{Z: 0, X: 0, Y: 0},
			expected: [4]float64{-180, -90, 0, 90},
		},
		"z0 east": {
			tile:     slippy.Tile{Z: 0, X: 1, Y: 0},
			expected: [4]float64{0, -90, 180, 90},
		},
		"z1 northwest": {
			tile:     slippy.Tile{Z: 1, X: 0, Y: 0},
			expected: [4]float64{-180, 0, -90, 90},
		},
		"z1 southwest": {
			tile:     slippy.Tile{Z: 1, X: 0, Y: 1},
			expected: [4]float64{-180, -90, -90, 0},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ext, err := tegola.WorldCRS84QuadExtent(tc.tile)
			if err != nil {
				t.Fatalf("WorldCRS84QuadExtent: %v", err)
			}
			if got := ext.Extent(); got != tc.expected {
				t.Fatalf("extent, expected %v got %v", tc.expected, got)
			}
		})
	}
}

func TestTileGridSizeWorldCRS84Quad(t *testing.T) {
	width, height, err := tegola.TileGridSize(tegola.WGS84, 1)
	if err != nil {
		t.Fatalf("TileGridSize: %v", err)
	}
	if width != 4 || height != 2 {
		t.Fatalf("size, expected 4x2 got %dx%d", width, height)
	}
}
