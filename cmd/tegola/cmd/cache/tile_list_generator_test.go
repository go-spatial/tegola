package cache

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-spatial/proj"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
)

func TestGenerateTilesForTileList(t *testing.T) {

	type tcase struct {
		// Set only one, TileFilename or TileList
		// if both is set TileFilename will take priority
		tileFilename string
		tileList     string
		format       Format
		zooms        []uint
		explicit     bool
		tiles        sTiles
		err          error
		maps         []atlas.Map
	}

	fn := func(tc tcase) (string, func(t *testing.T)) {
		name := tc.tileFilename
		if name == "" {
			h := sha1.New()
			io.WriteString(h, tc.tileList)
			name = fmt.Sprintf("internal string %x", h.Sum(nil))

		}
		return name, func(t *testing.T) {
			var in io.Reader
			if tc.tileFilename != "" {
				f, err := os.Open(tc.tileFilename)
				if err != nil {
					panic(fmt.Sprintf("unable to open testfile: %v", tc.tileFilename))
				}
				defer f.Close()
				in = f
			} else {
				in = strings.NewReader(tc.tileList)
			}

			tilechannel, err := generateTilesForTileList(context.Background(), in, tc.explicit, tc.zooms, tc.format, tc.maps)
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
		"test_3857_13/14/15 zooms": {
			tileFilename: "testdata/list.tiles",
			format:       defaultTileNameFormat,
			zooms:        []uint{13, 14, 15},
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
		"test_explicit_3857": {
			tileFilename: "testdata/list.tiles",
			format:       defaultTileNameFormat,
			explicit:     true,
			tiles: sTiles{
				slippy.NewTile(14, 300, 781),
			},
			maps: []atlas.Map{atlas.NewMap("test", proj.WebMercator)},
		},
		"test_4326_7/8/9 zooms": {
			tileFilename: "testdata/4326.tiles",
			format:       defaultTileNameFormat,
			zooms:        []uint{7, 8, 9},
			tiles: sTiles{
				slippy.NewTile(7, 250, 125),
				slippy.NewTile(8, 500, 250),
				slippy.NewTile(9, 1000, 500),
				slippy.NewTile(9, 1000, 501),
				slippy.NewTile(9, 1001, 500),
				slippy.NewTile(9, 1001, 501),
			},
			maps: []atlas.Map{atlas.NewMap("test", proj.EPSG4326)},
		},
	}
	for _, tc := range tests {
		t.Run(fn(tc))
	}

}
