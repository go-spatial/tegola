package cmd

import (
	"testing"
	"github.com/go-spatial/geom/slippy"
	"strings"
	"reflect"
	"fmt"
	"github.com/go-spatial/tegola"
)

// returns the index of the tile within the array, or -1
// if the tile is not in the array. This function dereferences the tiles
// in arr (in order to compare by value), therefore arr must not
// contain nil pointers
func searchTileArray(arr []*slippy.Tile, tile slippy.Tile) int {
	for k, v := range arr {
		if reflect.DeepEqual(*v, tile) {
			return k
		}
	}

	return -1
}

// creates a human readable string representation of
// tiles, used for debugging
func tilesString(tiles []*slippy.Tile) string {
	ret := "[ "
	for _, v := range tiles {
		ret += fmt.Sprint(v, " ")
	}
	ret += "]"
	return ret
}

func TestSendTiles (t *testing.T) {
	type tcase struct {
		flags string
		tiles []*slippy.Tile
	}

	fn := func(tc tcase, t *testing.T) {
		// parse flags
		err := cacheCmd.Flags().Parse(strings.Split(tc.flags, " "))
		if err != nil {
			t.Errorf("unexpected error %v", err)
			return
		}


		zooms, err := sliceFromRange(cacheMinZoom, cacheMaxZoom)
		if err != nil {
			t.Errorf("unexpected error %v", err)
			return
		}

		c := make(chan *slippy.Tile)

		go func () {
			sendTiles(zooms, c)
		}()

		// keep track of duplicates
		tileSet := map[string]struct{}{}
		tileDups := make([]slippy.Tile, 0)

		for tile := range c {
			if searchTileArray(tc.tiles, *tile) == -1 {
				t.Errorf("unexpected tile %v, expected %v", tile, tilesString(tc.tiles))
				return
			}

			// used as a map key
			tileString := fmt.Sprintf("%v", *tile)

			_, ok := tileSet[tileString]
			if ok {
				tileDups = append(tileDups, *tile)
			}
		}

		t.Logf("%d duplicated tiles\n", len(tileDups))
		// uncoment for debuging
		// t.Logf("%v", tileDups)
	}

	// note: the flags are left over from previous
	// test cases
	testcases := map[string]tcase{
		"max_zoom=0":{
			flags: "--min_zoom=0 --max_zoom=0 --bounds=\"-180,-85.0511,180,85.0511\"",
			tiles: []*slippy.Tile{
				slippy.NewTile(0, 0, 0, 0, tegola.WebMercator),
			},
		},
		"min_zoom=1 max_zoom=1":{
			flags: "--min_zoom=1 --max_zoom=1 --bounds=\"-180,-85.0511,180,85.0511\"",
			tiles: []*slippy.Tile{
				slippy.NewTile(1, 0, 0, 0, tegola.WebMercator),
				slippy.NewTile(1, 1, 0, 0, tegola.WebMercator),
				slippy.NewTile(1, 0, 1, 0, tegola.WebMercator),
				slippy.NewTile(1, 1, 1, 0, tegola.WebMercator),
			},
		},
		"min_zoom=1 max_zoom=1 bounds=180,90,0,0":{
			flags: "--min_zoom=1 --max_zoom=1 --bounds=\"180,90,0,0\"",
			tiles: []*slippy.Tile{
				slippy.NewTile(1, 1, 0, 0, tegola.WebMercator),
			},
		},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			fn(v, t)
		})
	}
}