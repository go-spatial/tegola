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
		expectedErr error
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
		{ // Confirm that negative column (x) value results in an error
			tile: tegola.Tile{
				Z: 2,
				X: -1,
				Y: 1,
			},
			expectedLat: 0.0,
			expectedLng: 0.0,
			expectedErr: tegola.ErrInvalidXY{
				X: -1,
				Y: 1,
			},
		},
		{ // Confirm that negative row (y) value results in an error
			tile: tegola.Tile{
				Z: 2,
				X: 1,
				Y: -1,
			},
			expectedLat: 0.0,
			expectedLng: 0.0,
			expectedErr: tegola.ErrInvalidXY{
				X: 1,
				Y: -1,
			},
		},
	}

	for i, test := range testcases {
		lat, lng, err := test.tile.Num2Deg()

		if test.expectedErr != nil {
			if err != test.expectedErr {
				t.Errorf("[%v]. Expected err (%v), got (%v)", i, test.expectedErr, err.Error())
				continue
			}
		}

		if lat != test.expectedLat {
			t.Errorf("[%v]. Expected lat (%v), got (%v)", i, test.expectedLat, lat)
			continue
		}
		if lng != test.expectedLng {
			t.Errorf("[%v]. Expected lng (%v), got (%v)", i, test.expectedLng, lng)
			continue
		}
	}
}

func TestTileDeg2Num(t *testing.T) {
	testcases := []struct {
		tile        tegola.Tile
		expectedX   int
		expectedY   int
		expectedErr error
	}{
		{
			tile: tegola.Tile{
				Z:    0,
				Lat:  -85,
				Long: -180,
			},
			expectedX: 0,
			expectedY: 0,
		},
		{ // Check for error if Lat outside WGS84 Range
			tile: tegola.Tile{
				Z:    0,
				Lat:  -85.1,
				Long: -180,
			},
			expectedX: 0,
			expectedY: 0,
			expectedErr: tegola.ErrInvalidLatLong{
				Lat:  -85.1,
				Long: -180,
			},
		},
		{ // Check for error if Long outside WGS84 Range
			tile: tegola.Tile{
				Z:    0,
				Lat:  -85,
				Long: -180.1,
			},
			expectedX: 0,
			expectedY: 0,
			expectedErr: tegola.ErrInvalidLatLong{
				Lat:  -85,
				Long: -180.1,
			},
		},
	}

	for i, tc := range testcases {
		x, y, err := tc.tile.Deg2Num()

		if tc.expectedErr != nil {
			if err == nil {
				t.Errorf("[%v]. got nil error but expected err: %v", i, tc.expectedErr)
				continue
			}
			if err != tc.expectedErr {
				t.Errorf("[%v]. expected err (%v) does not match output (%v)", i, tc.expectedErr, err)
				continue
			}
		}

		if tc.expectedX != x {
			t.Errorf("[%v]. expected X value (%v) does not match output (%v)", i, tc.expectedX, x)
			continue
		}

		if tc.expectedY != y {
			t.Errorf("[%v]. expected Y value (%v) does not match output (%v)", i, tc.expectedY, y)
			continue
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
