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

// Testing function passed to basic.ApplyToPoints.  Simply increments each coordinate value.
func inc(coords ...float64) ([]float64, error) {
	result := make([]float64, len(coords))
	for i, c := range coords {
		result[i] = c + 1.0
	}
	return result, nil
}

func TestApplyToPoints(t *testing.T) {
	// Acceptable floating point comparison fuzziness amount
	floatDelta := 0.000001

	type TestCase struct {
		geom         basic.Geometry
		expectedGeom basic.Geometry
	}

	testCases := []TestCase{
		{
			geom:         basic.Point{10.3, 11.7},
			expectedGeom: basic.Point{11.3, 12.7},
		},
		{
			geom:         basic.Point3{10.3, 11.7, 8},
			expectedGeom: basic.Point3{11.3, 12.7, 9},
		},
		{
			geom:         basic.Line{basic.Point{3.3, 3.3}, basic.Point{4.4, 4.4}, basic.Point{5.5, 5.5}},
			expectedGeom: basic.Line{basic.Point{4.3, 4.3}, basic.Point{5.4, 5.4}, basic.Point{6.5, 6.5}},
		},
		{
			geom: basic.Polygon{basic.Line{
				basic.Point{3.3, 3.3}, basic.Point{4.4, 4.4}, basic.Point{5.5, 5.5},
				basic.Point{5.5, 3.3}, basic.Point{3.3, 3.3}},
			},
			expectedGeom: basic.Polygon{basic.Line{
				basic.Point{4.3, 4.3}, basic.Point{5.4, 5.4}, basic.Point{6.5, 6.5},
				basic.Point{6.5, 4.3}, basic.Point{4.3, 4.3}},
			},
		},
		{
			geom:         basic.MultiPoint{basic.Point{10.10, 11.11}, basic.Point{11.11, 12.12}},
			expectedGeom: basic.MultiPoint{basic.Point{11.10, 12.11}, basic.Point{12.11, 13.12}},
		},
		{
			geom:         basic.MultiPoint3{basic.Point3{10.10, 11.11, 3}, basic.Point3{11.11, 12.12, 3}},
			expectedGeom: basic.MultiPoint3{basic.Point3{11.10, 12.11, 4}, basic.Point3{12.11, 13.12, 4}},
		},
		{
			geom: basic.MultiLine{
				basic.Line{basic.Point{3.3, 3.3}, basic.Point{4.4, 4.4}, basic.Point{5.5, 5.5}},
				basic.Line{basic.Point{6.6, 6.6}, basic.Point{5.5, 5.5}, basic.Point{4.4, 4.4}},
			},
			expectedGeom: basic.MultiLine{
				basic.Line{basic.Point{4.3, 4.3}, basic.Point{5.4, 5.4}, basic.Point{6.5, 6.5}},
				basic.Line{basic.Point{7.6, 7.6}, basic.Point{6.5, 6.5}, basic.Point{5.4, 5.4}},
			},
		},
		{
			geom: basic.MultiPolygon{
				basic.Polygon{
					basic.Line{
						basic.Point{1.1, 1.1}, basic.Point{2.2, 2.2}, basic.Point{3.3, 3.3},
						basic.Point{3.3, 1.1}, basic.Point{1.1, 1.1}},
				},
			},
			expectedGeom: basic.MultiPolygon{
				basic.Polygon{
					basic.Line{
						basic.Point{2.1, 2.1}, basic.Point{3.2, 3.2}, basic.Point{4.3, 4.3},
						basic.Point{4.3, 2.1}, basic.Point{2.1, 2.1}},
				},
			},
		},
	}

	for i, tc := range testCases {
		resultG, err := basic.ApplyToPoints(tc.geom, inc)
		if err != nil {
			t.Errorf("[%v] Problem with basic.ApplyToPoints(): %v", i, err)
		}
		switch {
		case resultG.IsPoint():
			resultPoint := resultG.AsPoint()
			expectedPoint := tc.expectedGeom.(basic.Point)
			if !basic.PointsEqual(resultPoint, expectedPoint, floatDelta) {
				t.Errorf("[%v] Point returned doesn't match expected: %v != %v", i, resultPoint, expectedPoint)
			}
		case resultG.IsPoint3():
			resultPoint := resultG.AsPoint3()
			expectedPoint := tc.expectedGeom.(basic.Point3)
			if !basic.Point3sEqual(resultPoint, expectedPoint, floatDelta) {
				t.Errorf("[%v] Point returned doesn't match expected: %v != %v", i, resultPoint, expectedPoint)
			}
		case resultG.IsLine():
			resultLine := resultG.AsLine()
			expectedLine := tc.expectedGeom.(basic.Line)
			if !basic.LinesEqual(resultLine, expectedLine, floatDelta) {
				t.Errorf("[%v] Line returned doesn't match expected: %v != %v", i, resultLine, expectedLine)
			}
		case resultG.IsPolygon():
			resultPoly := resultG.AsPolygon()
			expectedPoly := tc.expectedGeom.(basic.Polygon)
			if !basic.PolygonsEqual(resultPoly, expectedPoly, floatDelta) {
				t.Errorf("[%v] Polygon returned doesn't match expected: %v != %v", i, resultPoly, expectedPoly)
			}
		case resultG.IsMultiPoint():
			resultMP := resultG.AsMultiPoint()
			expectedMP := tc.expectedGeom.(basic.MultiPoint)
			if !basic.MultiPointsEqual(resultMP, expectedMP, floatDelta) {
				t.Errorf("[%v] MultiPoint returned doesn't match expected: %v != %v", i, resultMP, expectedMP)
			}
		case resultG.IsMultiPoint3():
			resultMP3 := resultG.AsMultiPoint3()
			expectedMP3 := tc.expectedGeom.(basic.MultiPoint3)
			if !basic.MultiPoint3sEqual(resultMP3, expectedMP3, floatDelta) {
				t.Errorf("[%v] MultiPoint3 returned doesn't match expected: %v != %v", i, resultMP3, expectedMP3)
			}
		case resultG.IsMultiLine():
			resultML := resultG.AsMultiLine()
			expectedML := tc.expectedGeom.(basic.MultiLine)
			if !basic.MultiLinesEqual(resultML, expectedML, floatDelta) {
				t.Errorf("[%v] MultiLine returned doesn't match expected: %v != %v", i, resultML, expectedML)
			}
		case resultG.IsMultiPolygon():
			resultMPoly := resultG.AsMultiPolygon()
			expectedMPoly := tc.expectedGeom.(basic.MultiPolygon)
			if !basic.MultiPolygonsEqual(resultMPoly, expectedMPoly, floatDelta) {
				t.Errorf("[%v] MultiPolygon returned doesn't match expected: %v != %v", i, resultMPoly, expectedMPoly)
			}
		}
	}
}
