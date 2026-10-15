package geometrycodec

import (
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
)

func TestDecodeRawWKTAllFamiliesAndMixed(t *testing.T) {
	fixtures := []struct {
		text string
		want geom.Geometry
	}{
		{"POINT (1 2)", geom.Point{1, 2}},
		{"POINT Z (1 2 -3)", geom.PointZ{1, 2, -3}},
		{"LINESTRING (1 2,3 4)", geom.LineString{{1, 2}, {3, 4}}},
		{"LINESTRING Z (1 2 3,4 5 6)", geom.LineStringZ{{1, 2, 3}, {4, 5, 6}}},
		{"POLYGON ((0 0,4 0,4 4,0 0),(1 1,2 1,2 2,1 1))", geom.Polygon{{{0, 0}, {4, 0}, {4, 4}, {0, 0}}, {{1, 1}, {2, 1}, {2, 2}, {1, 1}}}},
		{"POLYGON Z ((0 0 0,4 0 4,4 4 4,0 0 0),(1 1 1,2 1 2,2 2 2,1 1 1))", geom.PolygonZ{{{0, 0, 0}, {4, 0, 4}, {4, 4, 4}, {0, 0, 0}}, {{1, 1, 1}, {2, 1, 2}, {2, 2, 2}, {1, 1, 1}}}},
		{"MULTIPOINT (1 2,3 4)", geom.MultiPoint{{1, 2}, {3, 4}}},
		{"MULTIPOINT Z ((1 2 3),(4 5 6))", geom.MultiPointZ{{1, 2, 3}, {4, 5, 6}}},
		{"MULTILINESTRING ((1 2,3 4),EMPTY)", geom.MultiLineString{{{1, 2}, {3, 4}}, geom.LineString{}}},
		{"MULTILINESTRING Z ((1 2 3,4 5 6),EMPTY)", geom.MultiLineStringZ{{{1, 2, 3}, {4, 5, 6}}, geom.LineStringZ{}}},
		{"MULTIPOLYGON (((0 0,1 0,1 1,0 0)),EMPTY)", geom.MultiPolygon{{{{0, 0}, {1, 0}, {1, 1}, {0, 0}}}, geom.Polygon{}}},
		{"MULTIPOLYGON Z (((0 0 0,1 0 1,1 1 1,0 0 0)),EMPTY)", MultiPolygonZ{{{{0, 0, 0}, {1, 0, 1}, {1, 1, 1}, {0, 0, 0}}}, geom.PolygonZ{}}},
		{"GEOMETRYCOLLECTION (POINT (1 2),GEOMETRYCOLLECTION (POINT Z (3 4 5),POINT EMPTY))", geom.Collection{geom.Point{1, 2}, geom.Collection{geom.PointZ{3, 4, 5}, nil}}},
		{"GEOMETRYCOLLECTION Z (POINT (1 2 3),GEOMETRYCOLLECTION (LINESTRING (4 5 6,7 8 9)))", geom.Collection{geom.PointZ{1, 2, 3}, geom.Collection{geom.LineStringZ{{4, 5, 6}, {7, 8, 9}}}}},
		{"point z (+1e0 .2 -3.)", geom.PointZ{1, .2, -3}},
	}
	for _, tc := range fixtures {
		t.Run(tc.text, func(t *testing.T) {
			for _, input := range []any{tc.text, []byte(tc.text)} {
				got, err := DecodeRawWKT(input)
				if err != nil || !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("got%#v want%#v err%v", got, tc.want, err)
				}
			}
		})
	}
}

func TestDecodeRawWKTEmptyFamilies(t *testing.T) {
	fixtures := []struct {
		text string
		want geom.Geometry
	}{
		{"POINT EMPTY", nil}, {"POINT Z EMPTY", nil},
		{"LINESTRING EMPTY", geom.LineString{}}, {"LINESTRING Z EMPTY", geom.LineStringZ{}},
		{"POLYGON EMPTY", geom.Polygon{}}, {"POLYGON Z EMPTY", geom.PolygonZ{}},
		{"MULTIPOINT EMPTY", geom.MultiPoint{}}, {"MULTIPOINT Z EMPTY", geom.MultiPointZ{}},
		{"MULTILINESTRING EMPTY", geom.MultiLineString{}}, {"MULTILINESTRING Z EMPTY", geom.MultiLineStringZ{}},
		{"MULTIPOLYGON EMPTY", geom.MultiPolygon{}}, {"MULTIPOLYGON Z EMPTY", MultiPolygonZ{}},
		{"GEOMETRYCOLLECTION EMPTY", geom.Collection{}}, {"GEOMETRYCOLLECTION Z EMPTY", geom.Collection{}},
	}
	for _, tc := range fixtures {
		got, err := DecodeRawWKT(tc.text)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: %#v %v", tc.text, got, err)
		}
	}
}

func TestDecodeRawWKTRejectsMalformedAndGrammar(t *testing.T) {
	fixtures := []string{
		"", "POINT (1)", "POINT (1 2 3)", "POINT Z (1 2)", "POINT Z (1 2 3 4)",
		"POINT M (1 2 3)", "POINT ZM (1 2 3 4)", "SRID=4326;POINT(1 2)", "POINTZ(1 2 3)",
		"POINT (NaN 2)", "POINT (Inf 2)", "POINT Z (1 2 -Inf)", "POINT(0x1p2 2)", "POINT(1_000 2)", "POINT(1e 2)", "POINT(1e309 2)",
		"POINT(1 2) garbage", "POINT(1 2) POINT(3 4)", "POINT(1,2)", "POINT()", "POINT(1 2,3 4)",
		"LINESTRING(1 2)", "LINESTRING(1 2,)", "POLYGON(())", "POLYGON((0 0,1 1))",
		"POLYGON((0 0,1 0,1 1,0 1))", "POLYGON((0 0,1 1,0 0))",
		"POLYGON Z((0 0 0,1 0 1,1 1 1,0 1 0))", "POLYGON Z((0 0 0,1 1 1,0 0 0))",
		"POLYGON Z((0 0 0,1 0 1,1 1 1,0 0 9))",
		"MULTIPOINT(EMPTY)", "MULTIPOINT Z ((1 2 3),EMPTY)", "MULTIPOINT((1 2),(3 4 5))",
		"MULTILINESTRING((1 2,3 4),)", "MULTIPOLYGON(EMPTY,)", "GEOMETRYCOLLECTION()",
		"GEOMETRYCOLLECTION Z(POINT(1 2))", "GEOMETRYCOLLECTION(POINT(1 2),POINT M(1 2 3))",
	}
	for _, text := range fixtures {
		t.Run(text, func(t *testing.T) {
			if _, err := DecodeRawWKT(text); err == nil {
				t.Fatalf("invalidWKT accepted: %q", text)
			}
		})
	}
	if _, err := DecodeRawWKT(42); err == nil {
		t.Fatal("wrong source type accepted")
	}
	if got, err := DecodeRawWKT(nil); err != nil || got != nil {
		t.Fatal("SQL NULL rejected")
	}
}

func TestDecodeRawWKTDepthBudgetAndOwnership(t *testing.T) {
	text := "POINT Z(1 2 3)"
	for range rawMaxDepth {
		text = "GEOMETRYCOLLECTION(" + text + ")"
	}
	if _, err := DecodeRawWKT(text); err != nil {
		t.Fatalf("permitted depth rejected: %v", err)
	}
	if _, err := DecodeRawWKT("GEOMETRYCOLLECTION(" + text + ")"); err == nil {
		t.Fatal("depthoverflow accepted")
	}
	parser := rawWKTParser{text: "POINT EMPTY", elements: rawMaxElements}
	if _, err := parser.geometry(0, false); err == nil {
		t.Fatal("cumulative elementbudget ignored")
	}
	input := []byte("LINESTRING Z(1 2 3,4 5 6)")
	before := string(input)
	geometry, err := DecodeRawWKT(input)
	if err != nil {
		t.Fatal(err)
	}
	geometry.(geom.LineStringZ)[0][0] = 99
	if string(input) != before {
		t.Fatal("decoder output retained/mutated input")
	}
	value, err := DecodeRawWKT("POINT Z(-0 2 -3)")
	if err != nil || !math.Signbit(value.(geom.PointZ)[0]) {
		t.Fatal("signedzero lost")
	}
	if _, err := DecodeRawWKT(strings.Repeat(" ", rawMaxInputBytes+1)); err == nil {
		t.Fatal("input bytebudget ignored")
	}
}

func TestDecodeRawWKTBoundsUnsupportedNameError(t *testing.T) {
	_, err := DecodeRawWKT(strings.Repeat("x", 1<<20))
	if err == nil || len(err.Error()) > 200 {
		t.Fatal("unsupported name error echoed unbounded source content")
	}
}

func TestRawDimensionalUnsupportedClassification(t *testing.T) {
	for _, text := range []string{"UNKNOWN(1 2)", "POINT M (1 2 3)", "POINT ZM (1 2 3 4)", "MULTIPOINT(EMPTY)"} {
		_, err := DecodeRawWKT(text)
		if !errors.Is(err, ErrUnsupportedRawGeometry) {
			t.Errorf("unsupported WKT classification: %v", err)
		}
	}
	for _, code := range []uint32{2001, 3001, 0x80000001} {
		_, err := DecodeRawWKB(rawTestWKB(binary.LittleEndian, code, nil))
		if !errors.Is(err, ErrUnsupportedRawGeometry) {
			t.Errorf("unsupported WKB classification: %v", err)
		}
	}
	for _, value := range []any{"POINT(1)", "POLYGON((0 0,1 0,0 0))"} {
		_, err := DecodeRawWKT(value)
		if err == nil || errors.Is(err, ErrUnsupportedRawGeometry) {
			t.Errorf("malformed WKT misclassified: %v", err)
		}
	}
	_, err := DecodeRawWKB(rawTestWKB(binary.LittleEndian, 1, nil))
	if err == nil || errors.Is(err, ErrUnsupportedRawGeometry) {
		t.Errorf("truncated WKB misclassified: %v", err)
	}
}
