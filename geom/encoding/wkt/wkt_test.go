package wkt

import (
	"testing"

	"github.com/go-spatial/tegola/geom"
)

func TestEncode(t *testing.T) {
	type tcase struct {
		Geom geom.Geometry
		Rep  string
		Err  error
	}
	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		grep, gerr := Encode(tc.Geom)
		if tc.Err != nil {
			if tc.Err.Error() != gerr.Error() {
				t.Errorf("error, expected %v got %v", tc.Err.Error(), gerr.Error())
			}
			return
		}
		if tc.Err == nil && gerr != nil {
			t.Errorf("error, expected nil got %v", gerr)
			return
		}
		if tc.Rep != grep {
			t.Errorf("representation, expected ‘%v’ got ‘%v’", tc.Rep, grep)
		}

	}
	tests := map[string]map[string]tcase{
		"Point": {
			"empty nil": {
				Err: ErrUnknownGeometry{nil},
			},
			"empty": {
				Geom: (*geom.Point)(nil),
				Rep:  "POINT EMPTY",
			},
			"zero": {
				Geom: geom.Point{0, 0},
				Rep:  "POINT (0 0)",
			},
			"one": {
				Geom: geom.Point{10, 0},
				Rep:  "POINT (10 0)",
			},
		},
		"MultiPoint": {
			"empty nil": {
				Geom: (*geom.MultiPoint)(nil),
				Rep:  "MULTIPOINT EMPTY",
			},
			"empty zero": {
				Geom: geom.MultiPoint{},
				Rep:  "MULTIPOINT EMPTY",
			},
			"one": {
				Geom: geom.MultiPoint{{0, 0}},
				Rep:  "MULTIPOINT (0 0)",
			},
			"two": {
				Geom: geom.MultiPoint{{0, 0}, {10, 10}},
				Rep:  "MULTIPOINT (0 0,10 10)",
			},
			"three": {
				Geom: geom.MultiPoint{{1, 1}, {3, 3}, {4, 5}},
				Rep:  "MULTIPOINT (1 1,3 3,4 5)",
			},
		},
		"LineString": {
			"empty nil": {
				Geom: (*geom.LineString)(nil),
				Rep:  "LINESTRING EMPTY",
			},
			"empty zero": {
				Geom: geom.LineString{},
				Rep:  "LINESTRING EMPTY",
			},
			"one": {
				Geom: geom.LineString{{0, 0}},
				Rep:  "LINESTRING (0 0)",
			},
			"two": {
				Geom: geom.LineString{{10, 10}, {0, 0}},
				Rep:  "LINESTRING (10 10,0 0)",
			},
			"three": {
				Geom: geom.LineString{{10, 10}, {9, 9}, {0, 0}},
				Rep:  "LINESTRING (10 10,9 9,0 0)",
			},
		},
		"MultiLineString": {
			"empty nil": {
				Geom: (*geom.MultiLineString)(nil),
				Rep:  "MULTILINE EMPTY",
			},
			"zero lines": {
				Geom: geom.MultiLineString{},
				Rep:  "MULTILINE EMPTY",
			},
			"one line zero points": {
				Geom: geom.MultiLineString{{}},
				Rep:  "MULTILINE EMPTY",
			},
			"one line one point": {
				Geom: geom.MultiLineString{{{10, 10}}},
				Rep:  "MULTILINE ((10 10))",
			},
			"one line two points": {
				Geom: geom.MultiLineString{{{10, 10}, {11, 11}}},
				Rep:  "MULTILINE ((10 10,11 11))",
			},
			"two lines zero,zero point": {
				Geom: geom.MultiLineString{{}, {}},
				Rep:  "MULTILINE EMPTY",
			},
			"two lines zero,one point": {
				Geom: geom.MultiLineString{{}, {{10, 10}}},
				Rep:  "MULTILINE ((10 10))",
			},
			"two lines zero,two point": {
				Geom: geom.MultiLineString{{}, {{10, 10}, {20, 20}}},
				Rep:  "MULTILINE ((10 10,20 20))",
			},
			"two lines one,zero point": {
				Geom: geom.MultiLineString{{{10, 10}}, {}},
				Rep:  "MULTILINE ((10 10))",
			},
			"two lines one,one point": {
				Geom: geom.MultiLineString{{{10, 10}}, {{10, 10}}},
				Rep:  "MULTILINE ((10 10),(10 10))",
			},
			"two lines one,two point": {
				Geom: geom.MultiLineString{{{10, 10}}, {{10, 10}, {20, 20}}},
				Rep:  "MULTILINE ((10 10),(10 10,20 20))",
			},
			"two lines two,zero point": {
				Geom: geom.MultiLineString{{{10, 10}, {20, 20}}, {}},
				Rep:  "MULTILINE ((10 10,20 20))",
			},
			"two lines two,one point": {
				Geom: geom.MultiLineString{{{10, 10}, {20, 20}}, {{10, 10}}},
				Rep:  "MULTILINE ((10 10,20 20),(10 10))",
			},
			"two lines two,two point": {
				Geom: geom.MultiLineString{{{10, 10}, {20, 20}}, {{10, 10}, {20, 20}}},
				Rep:  "MULTILINE ((10 10,20 20),(10 10,20 20))",
			},
		},
		"Polygon": {
			"empty nil": {
				Geom: (*geom.Polygon)(nil),
				Rep:  "POLYGON EMPTY",
			},
			"empty": {
				Geom: geom.Polygon{},
				Rep:  "POLYGON EMPTY",
			},
			"one line zero": {
				Geom: geom.Polygon{{}},
				Rep:  "POLYGON EMPTY",
			},
			"two lines zero zero": {
				Geom: geom.Polygon{{}, {}},
				Rep:  "POLYGON EMPTY",
			},
			"two lines one zero": {
				Geom: geom.Polygon{{{10, 10}, {11, 11}, {12, 12}}, {}},
				Rep:  "POLYGON ((10 10,11 11,12 12))",
			},
			"two lines one one": {
				Geom: geom.Polygon{{{10, 10}, {11, 11}, {12, 12}}, {{20, 20}, {21, 21}, {22, 22}}},
				Rep:  "POLYGON ((10 10,11 11,12 12),(20 20,21 21,22 22))",
			},
			"two lines zero one": {
				Geom: geom.Polygon{{}, {{10, 10}, {11, 11}, {12, 12}}},
				Rep:  "POLYGON ((10 10,11 11,12 12))",
			},
		},
		"MultiPolygon": {
			"empty nil": {
				Geom: (*geom.MultiPolygon)(nil),
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty MultiPolygon": {
				Geom: geom.MultiPolygon{},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty one polygon": {
				Geom: geom.MultiPolygon{{}},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty one polygon one line": {
				Geom: geom.MultiPolygon{{{}}},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty two polygon 0": {
				Geom: geom.MultiPolygon{{}, {}},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty two polygon 1": {
				Geom: geom.MultiPolygon{{{}}, {}},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty two polygon 2": {
				Geom: geom.MultiPolygon{{}, {{}}},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"empty two polygon 3": {
				Geom: geom.MultiPolygon{{{}}, {{}}},
				Rep:  "MULTIPOLYGON EMPTY",
			},
			"one polygon": {
				Geom: geom.MultiPolygon{{{{10, 10}, {11, 11}, {12, 12}}}},
				Rep:  "MULTIPOLYGON (((10 10,11 11,12 12)))",
			},
		},
		"Collectioner": {
			"empty nil": {
				Geom: (*geom.Collection)(nil),
				Rep:  "GEOMETRYCOLLECTION EMPTY",
			},
			"empty": {
				Geom: geom.Collection{},
				Rep:  "GEOMETRYCOLLECTION EMPTY",
			},
			"empty nil point": {
				Geom: geom.Collection{
					(*geom.Point)(nil),
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty nil MultiPoint": {
				Geom: geom.Collection{
					(*geom.MultiPoint)(nil),
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty nil LineString": {
				Geom: geom.Collection{
					(*geom.LineString)(nil),
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty nil MultiLineString": {
				Geom: geom.Collection{
					(*geom.MultiLineString)(nil),
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty nil Polygon": {
				Geom: geom.Collection{
					(*geom.Polygon)(nil),
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty nil MultiPolygon": {
				Geom: geom.Collection{
					(*geom.MultiPolygon)(nil),
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty MultiPoint": {
				Geom: geom.Collection{
					geom.MultiPoint{},
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty LineString": {
				Geom: geom.Collection{
					geom.LineString{},
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty MultiLineString": {
				Geom: geom.Collection{
					geom.MultiLineString{},
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty Polygon": {
				Geom: geom.Collection{
					geom.Polygon{},
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"empty MultiPolygon": {
				Geom: geom.Collection{
					geom.MultiPolygon{},
				},
				Rep: "GEOMETRYCOLLECTION EMPTY",
			},
			"point": {
				Geom: geom.Collection{
					geom.Point{10, 10},
				},
				Rep: "GEOMETRYCOLLECTION (POINT (10 10))",
			},
			"point and linestring": {
				Geom: geom.Collection{
					geom.Point{10, 10},
					geom.LineString{{11, 11}, {22, 22}},
				},
				Rep: "GEOMETRYCOLLECTION (POINT (10 10),LINESTRING (11 11,22 22))",
			},
		},
	}
	for name, subtests := range tests {
		t.Run(name, func(t *testing.T) {
			for subname, tc := range subtests {
				tc := tc
				t.Run(subname, func(t *testing.T) { fn(t, tc) })
			}
		})
	}
}
