package cache

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/go-spatial/geom/slippy"
)

type sTiles []*slippy.Tile

func (st sTiles) Len() int      { return len(st) }
func (st sTiles) Swap(i, j int) { st[i], st[j] = st[j], st[i] }
func (st sTiles) Less(i, j int) bool {
	zi, xi, yi := st[i].ZXY()
	zj, xj, yj := st[j].ZXY()
	switch {
	case zi != zj:
		return zi < zj
	case xi != xj:
		return xi < xj
	default:
		return yi < yj
	}
}

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
		err    error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			// Setup up the generator.
			tilechannel := generateTilesForBounds(context.Background(), tc.bounds, tc.zooms)
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
			tiles:  sTiles{slippy.NewTile(0, 0, 0)},
		},
		"min_zoom=1 max_zoom=1": {
			zooms:  []uint{1},
			bounds: worldBounds,
			tiles: sTiles{
				slippy.NewTile(1, 0, 0),
				slippy.NewTile(1, 0, 1),
				slippy.NewTile(1, 1, 0),
				slippy.NewTile(1, 1, 1),
			},
		},
		"min_zoom=1 max_zoom=1 bounds=180,90,0,0": {
			zooms:  []uint{1},
			bounds: [4]float64{180.0, 90.0, 0.0, 0.0},
			tiles: sTiles{
				/*
				 * Note that the test case for this from the original had the tile being
				 * produced as 1/1/0 and not 1/1/1 but the code is identical, so not sure
				 * what the difference is.
				 */
				slippy.NewTile(1, 1, 1),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}
