package cache

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/proj"
)

type sTiles []slippy.Tile

func (st sTiles) Len() int           { return len(st) }
func (st sTiles) Swap(i, j int)      { st[i], st[j] = st[j], st[i] }
func (st sTiles) Less(i, j int) bool { return st[i].Less(st[j]) }

// IsEqual report true only if both the size and the elements are the same. Where a tile is equal only if the z,x,y elements match.
func (st sTiles) IsEqual(ost sTiles) bool {
	if len(st) != len(ost) {
		return false
	}
	for i := range st {
		zi, xi, yi := st[i].ZXY()
		zj, xj, yj := ost[i].ZXY()
		if zi != zj || xi != xj || yi != yj {
			return false
		}
	}
	return true
}

func (st sTiles) GoString() string {
	var b = bytes.NewBufferString("[")
	addComma := false
	for _, v := range st {
		if addComma {
			b.WriteString(",")
		} else {
			addComma = true
		}
		fmt.Fprintf(b, "%#v", v)
	}
	b.WriteString("]")
	return b.String()
}
func (st sTiles) String() string {
	var b = bytes.NewBufferString("[")
	b.WriteString("[")
	addComma := false
	for _, v := range st {
		if addComma {
			b.WriteString(",")
		} else {
			addComma = true
		}
		z, x, y := v.ZXY()
		fmt.Fprintf(b, "%v/%v/%v", z, x, y)
	}
	b.WriteString("]")
	return b.String()
}

func TestGenerateTilesForBounds(t *testing.T) {

	worldBounds := [4]float64{-180.0, -85.0511, 180, 85.0511}

	type tcase struct {
		zooms  []uint
		bounds [4]float64
		tiles  sTiles
		grid   slippy.TileGridder
		err    error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			// Setup up the generator.
			tilechannel := generateTilesForBounds(context.Background(), tc.bounds, tc.zooms, tc.grid)
			tiles := make(sTiles, 0, len(tc.tiles))
			for tile := range tilechannel.Channel() {
				tiles = append(tiles, tile)
			}
			if tc.err != nil {
				err := tilechannel.Err()
				if err == nil || err.Error() != tc.err.Error() {
					t.Errorf("error, expected %v got %v", err, tc.err)
				}
				// We expected an error so, don't trust the tiles.
				return
			}

			if err := tilechannel.Err(); err != nil {
				t.Errorf("error, expected nil got %v", err)
				return
			}

			sort.Sort(tiles)
			if !tc.tiles.IsEqual(tiles) {
				t.Errorf("unexpected tile list generated, expected %v got %v", tc.tiles, tiles)
			}
		}
	}

	tests := map[string]tcase{
		"max_zoom=0": {
			zooms:  []uint{0},
			bounds: worldBounds,
			tiles:  sTiles{slippy.Tile{}},
		},
		"min_zoom=1 max_zoom=1": {
			zooms:  []uint{1},
			bounds: worldBounds,
			tiles: sTiles{
				slippy.Tile{Z: 1},
				slippy.Tile{Z: 1, Y: 1},
				slippy.Tile{Z: 1, X: 1},
				slippy.Tile{Z: 1, X: 1, Y: 1},
			},
		},
		"min_zoom=1 max_zoom=1 bounds=180,90,0,0": {
			zooms:  []uint{1},
			bounds: [4]float64{180.0, 90.0, 0.0, 0.0},
			tiles: sTiles{
				slippy.Tile{Z: 1, X: 1},
				slippy.Tile{Z: 1, X: 1, Y: 1},
			},
		},
		"min_zoom=1 max_zoom=1 bounds=5.9,45.8,10.5,47.8 WSG84": {
			// see: https://github.com/go-spatial/tegola/issues/880#issuecomment-2556563251
			zooms:  []uint{10},
			bounds: [4]float64{5.9, 45.8, 10.5, 47.8},
			grid:   slippy.NewGrid(proj.EPSG4326, 0),
			tiles: sTiles{
				slippy.Tile{Z: 10, X: 528, Y: 356}, slippy.Tile{Z: 10, X: 528, Y: 357}, slippy.Tile{Z: 10, X: 528, Y: 358}, slippy.Tile{Z: 10, X: 528, Y: 359}, slippy.Tile{Z: 10, X: 528, Y: 360}, slippy.Tile{Z: 10, X: 528, Y: 361}, slippy.Tile{Z: 10, X: 528, Y: 362}, slippy.Tile{Z: 10, X: 528, Y: 363}, slippy.Tile{Z: 10, X: 528, Y: 364}, slippy.Tile{Z: 10, X: 528, Y: 365},
				slippy.Tile{Z: 10, X: 529, Y: 356}, slippy.Tile{Z: 10, X: 529, Y: 357}, slippy.Tile{Z: 10, X: 529, Y: 358}, slippy.Tile{Z: 10, X: 529, Y: 359}, slippy.Tile{Z: 10, X: 529, Y: 360}, slippy.Tile{Z: 10, X: 529, Y: 361}, slippy.Tile{Z: 10, X: 529, Y: 362}, slippy.Tile{Z: 10, X: 529, Y: 363}, slippy.Tile{Z: 10, X: 529, Y: 364}, slippy.Tile{Z: 10, X: 529, Y: 365},
				slippy.Tile{Z: 10, X: 530, Y: 356}, slippy.Tile{Z: 10, X: 530, Y: 357}, slippy.Tile{Z: 10, X: 530, Y: 358}, slippy.Tile{Z: 10, X: 530, Y: 359}, slippy.Tile{Z: 10, X: 530, Y: 360}, slippy.Tile{Z: 10, X: 530, Y: 361}, slippy.Tile{Z: 10, X: 530, Y: 362}, slippy.Tile{Z: 10, X: 530, Y: 363}, slippy.Tile{Z: 10, X: 530, Y: 364}, slippy.Tile{Z: 10, X: 530, Y: 365},
				slippy.Tile{Z: 10, X: 531, Y: 356}, slippy.Tile{Z: 10, X: 531, Y: 357}, slippy.Tile{Z: 10, X: 531, Y: 358}, slippy.Tile{Z: 10, X: 531, Y: 359}, slippy.Tile{Z: 10, X: 531, Y: 360}, slippy.Tile{Z: 10, X: 531, Y: 361}, slippy.Tile{Z: 10, X: 531, Y: 362}, slippy.Tile{Z: 10, X: 531, Y: 363}, slippy.Tile{Z: 10, X: 531, Y: 364}, slippy.Tile{Z: 10, X: 531, Y: 365},
				slippy.Tile{Z: 10, X: 532, Y: 356}, slippy.Tile{Z: 10, X: 532, Y: 357}, slippy.Tile{Z: 10, X: 532, Y: 358}, slippy.Tile{Z: 10, X: 532, Y: 359}, slippy.Tile{Z: 10, X: 532, Y: 360}, slippy.Tile{Z: 10, X: 532, Y: 361}, slippy.Tile{Z: 10, X: 532, Y: 362}, slippy.Tile{Z: 10, X: 532, Y: 363}, slippy.Tile{Z: 10, X: 532, Y: 364}, slippy.Tile{Z: 10, X: 532, Y: 365},
				slippy.Tile{Z: 10, X: 533, Y: 356}, slippy.Tile{Z: 10, X: 533, Y: 357}, slippy.Tile{Z: 10, X: 533, Y: 358}, slippy.Tile{Z: 10, X: 533, Y: 359}, slippy.Tile{Z: 10, X: 533, Y: 360}, slippy.Tile{Z: 10, X: 533, Y: 361}, slippy.Tile{Z: 10, X: 533, Y: 362}, slippy.Tile{Z: 10, X: 533, Y: 363}, slippy.Tile{Z: 10, X: 533, Y: 364}, slippy.Tile{Z: 10, X: 533, Y: 365},
				slippy.Tile{Z: 10, X: 534, Y: 356}, slippy.Tile{Z: 10, X: 534, Y: 357}, slippy.Tile{Z: 10, X: 534, Y: 358}, slippy.Tile{Z: 10, X: 534, Y: 359}, slippy.Tile{Z: 10, X: 534, Y: 360}, slippy.Tile{Z: 10, X: 534, Y: 361}, slippy.Tile{Z: 10, X: 534, Y: 362}, slippy.Tile{Z: 10, X: 534, Y: 363}, slippy.Tile{Z: 10, X: 534, Y: 364}, slippy.Tile{Z: 10, X: 534, Y: 365},
				slippy.Tile{Z: 10, X: 535, Y: 356}, slippy.Tile{Z: 10, X: 535, Y: 357}, slippy.Tile{Z: 10, X: 535, Y: 358}, slippy.Tile{Z: 10, X: 535, Y: 359}, slippy.Tile{Z: 10, X: 535, Y: 360}, slippy.Tile{Z: 10, X: 535, Y: 361}, slippy.Tile{Z: 10, X: 535, Y: 362}, slippy.Tile{Z: 10, X: 535, Y: 363}, slippy.Tile{Z: 10, X: 535, Y: 364}, slippy.Tile{Z: 10, X: 535, Y: 365},
				slippy.Tile{Z: 10, X: 536, Y: 356}, slippy.Tile{Z: 10, X: 536, Y: 357}, slippy.Tile{Z: 10, X: 536, Y: 358}, slippy.Tile{Z: 10, X: 536, Y: 359}, slippy.Tile{Z: 10, X: 536, Y: 360}, slippy.Tile{Z: 10, X: 536, Y: 361}, slippy.Tile{Z: 10, X: 536, Y: 362}, slippy.Tile{Z: 10, X: 536, Y: 363}, slippy.Tile{Z: 10, X: 536, Y: 364}, slippy.Tile{Z: 10, X: 536, Y: 365},
				slippy.Tile{Z: 10, X: 537, Y: 356}, slippy.Tile{Z: 10, X: 537, Y: 357}, slippy.Tile{Z: 10, X: 537, Y: 358}, slippy.Tile{Z: 10, X: 537, Y: 359}, slippy.Tile{Z: 10, X: 537, Y: 360}, slippy.Tile{Z: 10, X: 537, Y: 361}, slippy.Tile{Z: 10, X: 537, Y: 362}, slippy.Tile{Z: 10, X: 537, Y: 363}, slippy.Tile{Z: 10, X: 537, Y: 364}, slippy.Tile{Z: 10, X: 537, Y: 365},
				slippy.Tile{Z: 10, X: 538, Y: 356}, slippy.Tile{Z: 10, X: 538, Y: 357}, slippy.Tile{Z: 10, X: 538, Y: 358}, slippy.Tile{Z: 10, X: 538, Y: 359}, slippy.Tile{Z: 10, X: 538, Y: 360}, slippy.Tile{Z: 10, X: 538, Y: 361}, slippy.Tile{Z: 10, X: 538, Y: 362}, slippy.Tile{Z: 10, X: 538, Y: 363}, slippy.Tile{Z: 10, X: 538, Y: 364}, slippy.Tile{Z: 10, X: 538, Y: 365},
				slippy.Tile{Z: 10, X: 539, Y: 356}, slippy.Tile{Z: 10, X: 539, Y: 357}, slippy.Tile{Z: 10, X: 539, Y: 358}, slippy.Tile{Z: 10, X: 539, Y: 359}, slippy.Tile{Z: 10, X: 539, Y: 360}, slippy.Tile{Z: 10, X: 539, Y: 361}, slippy.Tile{Z: 10, X: 539, Y: 362}, slippy.Tile{Z: 10, X: 539, Y: 363}, slippy.Tile{Z: 10, X: 539, Y: 364}, slippy.Tile{Z: 10, X: 539, Y: 365},
				slippy.Tile{Z: 10, X: 540, Y: 356}, slippy.Tile{Z: 10, X: 540, Y: 357}, slippy.Tile{Z: 10, X: 540, Y: 358}, slippy.Tile{Z: 10, X: 540, Y: 359}, slippy.Tile{Z: 10, X: 540, Y: 360}, slippy.Tile{Z: 10, X: 540, Y: 361}, slippy.Tile{Z: 10, X: 540, Y: 362}, slippy.Tile{Z: 10, X: 540, Y: 363}, slippy.Tile{Z: 10, X: 540, Y: 364}, slippy.Tile{Z: 10, X: 540, Y: 365},
				slippy.Tile{Z: 10, X: 541, Y: 356}, slippy.Tile{Z: 10, X: 541, Y: 357}, slippy.Tile{Z: 10, X: 541, Y: 358}, slippy.Tile{Z: 10, X: 541, Y: 359}, slippy.Tile{Z: 10, X: 541, Y: 360}, slippy.Tile{Z: 10, X: 541, Y: 361}, slippy.Tile{Z: 10, X: 541, Y: 362}, slippy.Tile{Z: 10, X: 541, Y: 363}, slippy.Tile{Z: 10, X: 541, Y: 364}, slippy.Tile{Z: 10, X: 541, Y: 365},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}
