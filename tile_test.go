package tegola_test

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola"
)

func TestTileNum2Deg(t *testing.T) {
	testcases := []struct {
		tile        tegola.Tile
		expectedLat float64
		expectedLng float64
	}{
		{
			tile:        *tegola.NewTile(2, 1, 1),
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

func TestTileDeg2Num(t *testing.T) {
	testcases := []struct {
		tile      tegola.Tile
		expectedX int
		expectedY int
	}{
		{
			tile:      *tegola.NewTileLatLong(0, -85, -180),
			expectedX: 0,
			expectedY: 0,
		},
	}

	for i, tc := range testcases {
		x, y := tc.tile.Deg2Num()

		if tc.expectedX != x {
			t.Errorf("testcase (%v) failed. expected X value (%v) does not match output (%v)", i, tc.expectedX, x)
		}

		if tc.expectedY != y {
			t.Errorf("testcase (%v) failed. expected Y value (%v) does not match output (%v)", i, tc.expectedY, y)
		}
	}
}

func TestTileBBox(t *testing.T) {
	testcases := []struct {
		tile                   tegola.Tile
		minx, miny, maxx, maxy float64
	}{
		{
			tile: *tegola.NewTile(2, 1, 1),
			minx: -10018754.17,
			miny: 10018754.17,
			maxx: 0,
			maxy: 0,
		},
		{
			tile: *tegola.NewTile(16, 11436, 26461),
			minx: -13044437.497219238996,
			miny: 3856706.6986199953,
			maxx: -13043826.000993041,
			maxy: 3856095.202393799,
		},
	}

	for i, test := range testcases {
		bbox := test.tile.BoundingBox()

		if bbox.Minx != test.minx {
			t.Errorf("Failed test %v. Expected minx (%v), got (%v)", i, test.minx, bbox.Minx)
		}
		if bbox.Miny != test.miny {
			t.Errorf("Failed test %v. Expected miny (%v), got (%v)", i, test.miny, bbox.Miny)
		}
		if bbox.Maxx != test.maxx {
			t.Errorf("Failed test %v. Expected maxx (%v), got (%v)", i, test.maxx, bbox.Maxx)
		}
		if bbox.Maxy != test.maxy {
			t.Errorf("Failed test %v. Expected maxy (%v), got (%v)", i, test.maxy, bbox.Maxy)
		}
	}
}

func TestTileZRes(t *testing.T) {
	testcases := []struct {
		tile tegola.Tile
		zres float64
	}{
		{
			tile: *tegola.NewTile(2, 1, 1),
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

func TestTileToFromPixel(t *testing.T) {
	tile := tegola.NewTile(20, 0, 0)
	fn := func(idx int, pt [2]float64) {
		npt, err := tile.FromPixel(tegola.WebMercator, pt)
		if err != nil {
			t.Errorf("[%v] Error, Expected nil Got %v", idx, err)
			return
		}
		gpt, err := tile.ToPixel(tegola.WebMercator, npt)
		if err != nil {
			t.Errorf("[%v] Error, Expected nil Got %v", idx, err)
			return
		}
		// TODO: gdey need to find the utility math function for comparing floats.
		if pt[0] != gpt[0] {
			t.Errorf("[%v] unequal x value, Expected %v Got %v", idx, pt[0], gpt[0])
		}
		if pt[1] != gpt[1] {
			t.Errorf("[%v] unequal y value, Expected %v Got %v", idx, pt[0], gpt[0])
		}

	}
	tbltest.Cases(
		[2]float64{1, 1},
		[2]float64{0, 0},
		[2]float64{4000, 4000},
	).Run(fn)
}
