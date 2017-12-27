package basic_test

import (
	"fmt"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/wkb"
)

func TestToWebMercator(t *testing.T) {
	var floatDelta float64 = 0.00001 // floating point comparison fuzziness amount
	unsupportedSrid := 3157

	type TestCase struct {
		fromSrid    int // Currently only WGS84 -> WebMercator is supported
		wgs84G      basic.Geometry
		expectedG   basic.Geometry
		expectedErr string
	}

	testCases := []TestCase{
		{
			fromSrid:  unsupportedSrid,
			wgs84G:    basic.Point{-80.0, -40.0},
			expectedG: basic.G{},
			expectedErr: fmt.Sprintf("don't know how to convert from %v to %v.",
				unsupportedSrid, tegola.WebMercator),
		},
		{
			fromSrid: tegola.WGS84,
			wgs84G: basic.Line{
				basic.Point{-80.0, -40.0},
				basic.Point{-40.0, -40.0},
				basic.Point{-40.0, 0.0},
				basic.Point{-80.0, 0.0},
				basic.Point{-80.0, -40.0},
			},
			// What it should be after conversion, according to https://mygeodata.cloud/cs2cs/
			expectedG: basic.Line{
				basic.Point{-8905559.26346, -4865942.2795},
				basic.Point{-4452779.63173, -4865942.2795},
				basic.Point{-4452779.63173, -7.08115455161e-10},
				basic.Point{-8905559.26346, -7.08115455161e-10},
				basic.Point{-8905559.26346, -4865942.2795},
			},
		},
		{
			fromSrid: tegola.WGS84,
			wgs84G: basic.Polygon{
				basic.Line{
					basic.Point{-80.0, -40.0},
					basic.Point{-40.0, -40.0},
					basic.Point{-40.0, 0.0},
					basic.Point{-80.0, 0.0},
					basic.Point{-80.0, -40.0},
				},
			},
			// What it should be after conversion, according to https://mygeodata.cloud/cs2cs/
			expectedG: basic.Polygon{
				basic.Line{
					basic.Point{-8905559.26346, -4865942.2795},
					basic.Point{-4452779.63173, -4865942.2795},
					basic.Point{-4452779.63173, -7.08115455161e-10},
					basic.Point{-8905559.26346, -7.08115455161e-10},
					basic.Point{-8905559.26346, -4865942.2795},
				},
			},
		},
		{
			fromSrid: tegola.WGS84,
			wgs84G: basic.Polygon{
				basic.Line{
					basic.Point{75.0, -40.0},
					basic.Point{70.0, 10.0},
					basic.Point{75.0, 30.0},
					basic.Point{40.0, 35.0},
					basic.Point{-10.0, 10.0},
					basic.Point{-5.3, -25.7},
					basic.Point{75, -40.0},
				},
			},
			// What it should be after conversion, according to https://mygeodata.cloud/cs2cs/
			expectedG: basic.Polygon{
				basic.Line{
					basic.Point{8348961.8095, -4865942.2795},
					basic.Point{7792364.35553, 1118889.97486},
					basic.Point{8348961.8095, 3503549.8435},
					basic.Point{4452779.63173, 4163881.14406},
					basic.Point{-1113194.90793, 1118889.97486},
					basic.Point{-589993.301204, -2961971.85332},
					basic.Point{8348961.8095, -4865942.2795},
				},
			},
		},
	}

	for i, tc := range testCases {
		resultG, err := basic.ToWebMercator(tc.fromSrid, tc.wgs84G)
		if err != nil {
			if tc.expectedErr == "" {
				t.Errorf("[%v] Problem in basic.ToWebMercator(): %v\n", i, err)
				continue
			} else if tc.expectedErr != err.Error() {
				t.Errorf("[%v] expected error not returned: '%v' != '%v'", i, err.Error(), tc.expectedErr)
			}
		}

		switch {
		case resultG.IsLine():
			resultLine := resultG.AsLine()
			expectedLine := tc.expectedG.(basic.Line)

			for j, p := range resultLine {
				if !basic.PointsEqual(p, expectedLine[j], floatDelta) {
					t.Errorf("[%v] (Line [%v])\n  %v\n --- != ---\n  %v\n", i, j, p, expectedLine[j])
				}
			}
		case resultG.IsPolygon():
			resultPolygon := resultG.AsPolygon()
			expectedPoly := tc.expectedG.(basic.Polygon)

			for j, l := range resultPolygon {
				expectedLine := expectedPoly.Sublines()[j].(basic.Line)
				if !basic.LinesEqual(l, expectedLine, floatDelta) {
					t.Errorf("[%v] (Polygon [%v])\n  %v\n --- != ---\n  %v\n", i, j, wkb.WKT(l), wkb.WKT(expectedLine))
				}
			}
		}
	}
}

func TestFromWebMercator(t *testing.T) {
	var floatDelta float64 = 0.00001 // floating point comparison fuzziness amount
	unsupportedSrid := 3157
	type TestCase struct {
		toSrid      int // Currently only WebMercator -> WGS84 is supported
		webMerc     basic.Geometry
		expectedG   basic.Geometry
		expectedErr string
	}

	testCases := []TestCase{
		{
			toSrid:    unsupportedSrid,
			webMerc:   basic.Point{-8905559.26346, -4865942.2795},
			expectedG: basic.G{},
			expectedErr: fmt.Sprintf("don't know how to convert from %v to %v.",
				tegola.WebMercator, unsupportedSrid),
		},
		{
			toSrid: tegola.WGS84,
			webMerc: basic.Line{
				basic.Point{-8905559.26346, -4865942.2795},
				basic.Point{-4452779.63173, -4865942.2795},
				basic.Point{-4452779.63173, -7.08115455161e-10},
				basic.Point{-8905559.26346, -7.08115455161e-10},
				basic.Point{-8905559.26346, -4865942.2795},
			},
			// What it should be after conversion, according to https://mygeodata.cloud/cs2cs/
			expectedG: basic.Line{
				basic.Point{-80.0, -40.0},
				basic.Point{-40.0, -40.0},
				basic.Point{-40.0, 0.0},
				basic.Point{-80.0, 0.0},
				basic.Point{-80.0, -40.0},
			},
		},
		{
			toSrid: tegola.WGS84,
			webMerc: basic.Polygon{
				basic.Line{
					basic.Point{-8905559.26346, -4865942.2795},
					basic.Point{-4452779.63173, -4865942.2795},
					basic.Point{-4452779.63173, -7.08115455161e-10},
					basic.Point{-8905559.26346, -7.08115455161e-10},
					basic.Point{-8905559.26346, -4865942.2795},
				},
			},
			// What it should be after conversion, according to https://mygeodata.cloud/cs2cs/
			expectedG: basic.Polygon{
				basic.Line{
					basic.Point{-80.0, -40.0},
					basic.Point{-40.0, -40.0},
					basic.Point{-40.0, 0.0},
					basic.Point{-80.0, 0.0},
					basic.Point{-80.0, -40.0},
				},
			},
		},
		{
			toSrid: tegola.WGS84,
			webMerc: basic.Polygon{
				basic.Line{
					basic.Point{8348961.8095, -4865942.2795},
					basic.Point{7792364.35553, 1118889.97486},
					basic.Point{8348961.8095, 3503549.8435},
					basic.Point{4452779.63173, 4163881.14406},
					basic.Point{-1113194.90793, 1118889.97486},
					basic.Point{-589993.301204, -2961971.85332},
					basic.Point{8348961.8095, -4865942.2795},
				},
			},
			// What it should be after conversion, according to https://mygeodata.cloud/cs2cs/
			expectedG: basic.Polygon{
				basic.Line{
					basic.Point{75.0, -40.0},
					basic.Point{70.0, 10.0},
					basic.Point{75.0, 30.0},
					basic.Point{40.0, 35.0},
					basic.Point{-10.0, 10.0},
					basic.Point{-5.3, -25.7},
					basic.Point{75, -40.0},
				},
			},
		},
	}

	for i, tc := range testCases {
		resultG, err := basic.FromWebMercator(tc.toSrid, tc.webMerc)
		if err != nil {
			if tc.expectedErr == "" {
				t.Errorf("[%v] Problem in basic.FromWebMercator(): %v\n", i, err)
				continue
			} else {
				if tc.expectedErr != err.Error() {
					t.Errorf("[%v] Expected error not returned: '%v' != '%v'", i, err, tc.expectedErr)
				}
			}
		}

		switch {
		case resultG.IsLine():
			resultLine := resultG.AsLine()
			expectedLine := tc.expectedG.(basic.Line)

			for j, p := range resultLine {
				if !basic.PointsEqual(p, expectedLine[j], floatDelta) {
					t.Errorf("[%v] (Line [%v])\n  %v\n --- != ---\n  %v\n", i, j, p, expectedLine[j])
				}
			}
		case resultG.IsPolygon():
			resultPolygon := resultG.AsPolygon()
			expectedPoly := tc.expectedG.(basic.Polygon)

			for j, l := range resultPolygon {
				expectedLine := expectedPoly.Sublines()[j].(basic.Line)
				if !basic.LinesEqual(l, expectedLine, floatDelta) {
					t.Errorf("[%v] (Polygon [%v])\n  %v\n --- != ---\n  %v\n", i, j, wkb.WKT(l), wkb.WKT(expectedLine))
				}
			}
		}
	}
}
