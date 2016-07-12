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
			t.Errorf("Failed test %v. Expected lat (%v), got (%v)", i, test.expectedLat, lat)
		}
		if lng != test.expectedLng {
			t.Errorf("Failed test %v. Expected lng (%v), got (%v)", i, test.expectedLng, lng)
		}
	}
}

func TestTileBBox(t *testing.T) {
	testcases := []struct {
		tile                   tegola.Tile
		minx, miny, maxx, maxy float64
	}{
		{
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: 1,
			},
			minx: -10018754.17,
			miny: 10018754.17,
			maxx: 0,
			maxy: 0,
		},
		{
			tile: tegola.Tile{
				Z: 16,
				X: 11436,
				Y: 26461,
			},
			minx: -13044437.497219238996,
			miny: 3856706.6986199953,
			maxx: -13043826.000993041,
			maxy: 3856095.202393799,
		},
	}

	for i, test := range testcases {
		minx, miny, maxx, maxy := test.tile.BBox()

		if minx != test.minx {
			t.Errorf("Failed test %v. Expected minx (%v), got (%v)", i, test.minx, minx)
		}
		if miny != test.miny {
			t.Errorf("Failed test %v. Expected miny (%v), got (%v)", i, test.miny, miny)
		}
		if maxx != test.maxx {
			t.Errorf("Failed test %v. Expected maxx (%v), got (%v)", i, test.maxx, maxx)
		}
		if maxy != test.maxy {
			t.Errorf("Failed test %v. Expected maxy (%v), got (%v)", i, test.maxy, maxy)
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
