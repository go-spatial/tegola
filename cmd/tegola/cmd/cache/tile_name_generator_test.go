package cache

import (
	"context"
	"sort"
	"testing"

	"github.com/go-spatial/proj"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
)

func TestGenerateTilesForTileName(t *testing.T) {

	type tcase struct {
		tile     *slippy.Tile
		zooms    []uint
		explicit bool
		tiles    sTiles
		err      error
		maps     []atlas.Map
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			tilechannel, err := generateTilesForTileName(context.Background(), tc.tile, tc.explicit, tc.zooms, tc.maps)
			if err != nil {
				t.Errorf("%v %v", t.Name(), err)
				return
			}

			tiles := make([]*MapTile, 0, len(tc.tiles))
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

			stiles := make(sTiles, 0, len(tc.tiles))
			for _, t := range tiles {
				stiles = append(stiles, t.Tile)
			}

			sort.Sort(stiles)
			if !tc.tiles.IsEqual(stiles) {
				t.Errorf("unexpected tile list generated, expected %v got %v", tc.tiles, stiles)
			}
		}
	}

	tests := map[string]tcase{
		"max_zoom=0 tile-name=0/0/0": {
			tile:     slippy.NewTile(0, 0, 0),
			explicit: true,
			tiles: sTiles{
				slippy.NewTile(0, 0, 0),
			},
			maps: []atlas.Map{atlas.NewMap("test", proj.WebMercator)},
		},
		"max_zoom=0 tile-name=14/300/781": {
			tile:     slippy.NewTile(14, 300, 781),
			explicit: true,
			tiles: sTiles{
				slippy.NewTile(14, 300, 781),
			},
			maps: []atlas.Map{atlas.NewMap("test", proj.WebMercator)},
		},
		"min_zoom= 13 max_zoom=15 tile-name=14/300/781": {
			tile:  slippy.NewTile(14, 300, 781),
			zooms: []uint{13, 14, 15},
			tiles: sTiles{
				slippy.NewTile(13, 150, 390),
				slippy.NewTile(14, 300, 781),
				slippy.NewTile(15, 600, 1562),
				slippy.NewTile(15, 600, 1563),
				slippy.NewTile(15, 601, 1562),
				slippy.NewTile(15, 601, 1563),
			},
			maps: []atlas.Map{atlas.NewMap("test", proj.WebMercator)},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}
