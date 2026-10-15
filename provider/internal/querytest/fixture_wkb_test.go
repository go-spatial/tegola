package querytest

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestFixtureWKBIndependentByteGoldens(t *testing.T) {
	for _, tc := range []struct {
		name     string
		geometry geom.Geometry
		hex      string
	}{
		{"XY point", geom.Point{1, 2}, "0101000000000000000000f03f0000000000000040"},
		{"XYZ point", geom.PointZ{1, 2, 3}, "01e9030000000000000000f03f00000000000000400000000000000840"},
		{"XYZ empty line", geom.LineStringZ{}, "01ea03000000000000"},
		{"mixed collection", geom.Collection{geom.Point{1, 2}, geom.PointZ{1, 2, 3}},
			"0107000000020000000101000000000000000000f03f000000000000004001e9030000000000000000f03f00000000000000400000000000000840"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, err := hex.DecodeString(tc.hex)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EncodeFixtureWKB(tc.geometry)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("wire %x want %x", got, want)
			}
		})
	}
}

func TestFixtureWKBDeclaredFamilyHeaders(t *testing.T) {
	for _, tc := range []struct {
		geometry geom.Geometry
		code     uint32
		child    uint32
	}{
		{geom.LineString{{1, 2}, {3, 4}}, 2, 0},
		{geom.Polygon{{{0, 0}, {1, 0}, {0, 1}, {0, 0}}}, 3, 0},
		{geom.MultiPoint{{1, 2}}, 4, 1},
		{geom.MultiLineString{{{1, 2}, {3, 4}}}, 5, 2},
		{geom.MultiPolygon{{{{0, 0}, {1, 0}, {0, 1}, {0, 0}}}}, 6, 3},
		{geom.LineStringZ{{1, 2, 3}, {4, 5, 6}}, 1002, 0},
		{geom.PolygonZ{{{0, 0, 1}, {1, 0, 1}, {0, 1, 1}, {0, 0, 1}}}, 1003, 0},
		{geom.MultiPointZ{{1, 2, 3}}, 1004, 1001},
		{geom.MultiLineStringZ{{{1, 2, 3}, {4, 5, 6}}}, 1005, 1002},
		{geometrycodec.MultiPolygonZ{{{{0, 0, 1}, {1, 0, 1}, {0, 1, 1}, {0, 0, 1}}}}, 1006, 1003},
	} {
		wire, err := EncodeFixtureWKB(tc.geometry)
		if err != nil {
			t.Fatal(err)
		}
		if wire[0] != 1 || binary.LittleEndian.Uint32(wire[1:5]) != tc.code {
			t.Fatalf("wrong header %x", wire[:5])
		}
		if tc.child != 0 && (binary.LittleEndian.Uint32(wire[5:9]) != 1 ||
			wire[9] != 1 || binary.LittleEndian.Uint32(wire[10:14]) != tc.child) {
			t.Fatal("lost child family")
		}
	}
	for _, g := range []geom.Geometry{nil, geom.PointM{1, 2, 3}, geom.PointZM{1, 2, 3, 4}} {
		if _, err := EncodeFixtureWKB(g); err == nil {
			t.Fatal("unsupported family accepted")
		}
	}
	var nested geom.Geometry = geom.Point{1, 2}
	for i := 0; i < 32; i++ {
		nested = geom.Collection{nested}
	}
	if _, err := EncodeFixtureWKB(nested); err == nil {
		t.Fatal("depth guard missing")
	}
}
