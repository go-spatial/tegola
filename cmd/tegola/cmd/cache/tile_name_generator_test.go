package cache

import (
	"context"
	"sort"
	"testing"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
)

func TestGenerateTilesForTileName(t *testing.T) {

	type tcase struct {
		tile     *slippy.Tile
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
			tile:     slippy.NewTile(0, 0, 0, 0, tegola.WebMercator),
			explicit: true,
			tiles: sTiles{
				slippy.NewTile(0, 0, 0, 0, tegola.WebMercator),
			},
		},
		"max_zoom=0 tile-name=14/300/781": {
			tile:     slippy.NewTile(14, 300, 781, 0, tegola.WebMercator),
			explicit: true,
			tiles: sTiles{
				slippy.NewTile(14, 300, 781, 0, tegola.WebMercator),
			},
		},
		"min_zoom= 13 max_zoom=15 tile-name=14/300/781": {
			tile:  slippy.NewTile(14, 300, 781, 0, tegola.WebMercator),
			zooms: []uint{13, 14, 15},
			tiles: sTiles{
				slippy.NewTile(13, 150, 390, 0, tegola.WebMercator),
				slippy.NewTile(14, 300, 781, 0, tegola.WebMercator),
				slippy.NewTile(15, 600, 1562, 0, tegola.WebMercator),
				slippy.NewTile(15, 600, 1563, 0, tegola.WebMercator),
				slippy.NewTile(15, 601, 1562, 0, tegola.WebMercator),
				slippy.NewTile(15, 601, 1563, 0, tegola.WebMercator),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}
