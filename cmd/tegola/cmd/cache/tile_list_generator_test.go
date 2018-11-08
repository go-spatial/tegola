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

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
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

			tilechannel := generateTilesForTileList(context.Background(), in, tc.explicit, tc.zooms, tc.format)
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

	tests := [...]tcase{
		{
			tileFilename: "testdata/list.tiles",
			format:       defaultTileNameFormat,
			zooms:        []uint{13, 14, 15},
			tiles: sTiles{
				slippy.NewTile(13, 150, 390, 0, tegola.WebMercator),
				slippy.NewTile(14, 300, 781, 0, tegola.WebMercator),
				slippy.NewTile(15, 600, 1562, 0, tegola.WebMercator),
				slippy.NewTile(15, 600, 1563, 0, tegola.WebMercator),
				slippy.NewTile(15, 601, 1562, 0, tegola.WebMercator),
				slippy.NewTile(15, 601, 1563, 0, tegola.WebMercator),
			},
		},
		{
			tileFilename: "testdata/list.tiles",
			format:       defaultTileNameFormat,
			explicit:     true,
			tiles: sTiles{
				slippy.NewTile(14, 300, 781, 0, tegola.WebMercator),
			},
		},
	}
	for _, tc := range tests {
		t.Run(fn(tc))
	}

}
