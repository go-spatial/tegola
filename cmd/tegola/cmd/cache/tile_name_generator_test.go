package cache

import (
	"context"
	"sort"
	"testing"

	"github.com/go-spatial/geom/slippy"
)

func TestGenerateTilesForTileName(t *testing.T) {

	type tcase struct {
		tile     slippy.Tile
		zooms    []uint
		explicit bool
		tiles    sTiles
		err      error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			tilechannel := generateTilesForTileName(context.Background(), tc.tile, tc.explicit, tc.zooms)
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
		"max_zoom=0 tile-name=0/0/0": {
			tile:     slippy.Tile{},
			explicit: true,
			tiles: sTiles{
				slippy.Tile{},
			},
		},
		"max_zoom=0 tile-name=14/300/781": {
			tile:     slippy.Tile{Z: 14, X: 300, Y: 781},
			explicit: true,
			tiles: sTiles{
				slippy.Tile{Z: 14, X: 300, Y: 781},
			},
		},
		"min_zoom= 13 max_zoom=15 tile-name=14/300/781": {
			tile:  slippy.Tile{Z: 14, X: 300, Y: 781},
			zooms: []uint{13, 14, 15},
			tiles: sTiles{
				slippy.Tile{Z: 13, X: 150, Y: 390},
				slippy.Tile{Z: 14, X: 300, Y: 781},
				slippy.Tile{Z: 15, X: 600, Y: 1562},
				slippy.Tile{Z: 15, X: 600, Y: 1563},
				slippy.Tile{Z: 15, X: 601, Y: 1562},
				slippy.Tile{Z: 15, X: 601, Y: 1563},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}
