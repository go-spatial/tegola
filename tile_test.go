package tegola_test

import (
	"testing"

	"github.com/terranodo/tegola"
)

func TestTileNum2Deg(t *testing.T) {
	testcases := []struct {
		tile        tegola.Tile
		expectedLat float64
		expectedLng float64
	}{
		{
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			expectedLat: 66.51326044311185,
			expectedLng: -90,
		},
	}

	for i, test := range testcases {
		lat, lng := test.tile.Num2Deg()
		if lat != test.expectedLat {
			t.Errorf("Failed test %v. Expected lat (%v), got (%v)", i, lat, lat)
		}
		if lng != test.expectedLng {
			t.Errorf("Failed test %v. Expected lng (%v), got (%v)", i, lng, lng)
		}
	}
}

func TestTileBBox(t *testing.T) {
	testcases := []struct {
		tile               tegola.Tile
		ulx, uly, llx, lly float64
	}{
		{
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			ulx: -10018754.17,
			uly: 10018754.17,
			llx: 0,
			lly: 0,
		},
	}

	for i, test := range testcases {
		ulx, uly, llx, lly := test.tile.BBox()

		if ulx != test.ulx {
			t.Errorf("Failed test %v. Expected ulx (%v), got (%v)", i, test.ulx, ulx)
		}
		if uly != test.uly {
			t.Errorf("Failed test %v. Expected uly (%v), got (%v)", i, test.uly, uly)
		}
		if llx != test.llx {
			t.Errorf("Failed test %v. Expected llx (%v), got (%v)", i, test.llx, llx)
		}
		if lly != test.lly {
			t.Errorf("Failed test %v. Expected lly (%v), got (%v)", i, test.lly, lly)
		}
	}
}

func TestTileZRes(t *testing.T) {
	testcases := []struct {
		tile tegola.Tile
		zres float64
	}{
		{
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			zres: 39135.75848201026,
		},
	}

	for i, test := range testcases {
		zres := test.tile.ZRes()

		if zres != test.zres {
			t.Errorf("Failed test %v. Expected zres (%v), got (%v)", i, test.zres, zres)
		}
	}
}
