package tegola_test

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola"
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

func TestTileZRes(t *testing.T) {
	testcases := []struct {
		tile tegola.Tile
		zres float64
	}{
		{
			tile: *tegola.NewTile(2, 1, 1),
			// this is for 4096x4096
			zres: 2445.984905125641,
			// this is for 256x256
			// zres: 39135.75848201026,
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
