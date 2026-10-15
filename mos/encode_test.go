package mos

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
)

func TestEncodeRoundTrip(t *testing.T) {
	opts := Options{Precision: 3} // kPrecision = 1000
	cases := []struct {
		name string
		g    geom.Geometry
	}{
		{"point", geom.Point{10.1234, 20.5678}},
		{"multipoint", geom.MultiPoint{{1, 2}, {3, 4}}},
		{"linestring", geom.LineString{{0, 0}, {10.5, 20.25}}},
		{"multilinestring", geom.MultiLineString{{{0, 0}, {1, 1}}, {{2, 2}, {3, 3}}}},
		{"polygon", geom.Polygon{{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}}},
		{"polygon-hole", geom.Polygon{
			{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
			{{2, 2}, {4, 2}, {4, 4}, {2, 4}, {2, 2}},
		}},
		{"multipolygon", geom.MultiPolygon{
			{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
			{{{5, 5}, {6, 5}, {6, 6}, {5, 6}, {5, 5}}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			blob, err := Encode(c.g, opts)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			// Native 12-byte header.
			if len(blob) < 12 {
				t.Fatalf("blob too short: %d", len(blob))
			}
			back, err := Decode(blob, opts)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if !geomsApproxEqual(c.g, back, 0.001) {
				t.Fatalf("round trip mismatch:\n in: %#v\nout: %#v", c.g, back)
			}
		})
	}
}

func TestEncodeHeaderLayout(t *testing.T) {
	// Mirrors THeaderObject: 12-byte header, counts, then int32 pairs.
	blob, err := Encode(geom.Point{1.5, -2.5}, Options{Precision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(blob) != 12+4+8 {
		t.Fatalf("expected 24 bytes, got %d", len(blob))
	}
	if blob[0] != TypePoint {
		t.Fatalf("oType = %d, want %d", blob[0], TypePoint)
	}
	// subObjectsCount at offset 4 (uint16 LE) = 1
	if blob[4] != 1 || blob[5] != 0 {
		t.Fatalf("subObjectsCount bytes = %v", blob[4:6])
	}
	// pointsCount at offset 6 (int32 LE) = 1
	if blob[6] != 1 || blob[7] != 0 || blob[8] != 0 || blob[9] != 0 {
		t.Fatalf("pointsCount bytes = %v", blob[6:10])
	}
	// ofl at offset 10 = 0
	if blob[10] != 0 || blob[11] != 0 {
		t.Fatalf("ofl bytes = %v", blob[10:12])
	}
	// first subobject count at offset 12 = 1
	if blob[12] != 1 || blob[13] != 0 || blob[14] != 0 || blob[15] != 0 {
		t.Fatalf("count bytes = %v", blob[12:16])
	}
	// point (150, -250) as int32 LE
	x := int32(blob[16]) | int32(blob[17])<<8 | int32(blob[18])<<16 | int32(blob[19])<<24
	y := int32(blob[20]) | int32(blob[21])<<8 | int32(blob[22])<<16 | int32(blob[23])<<24
	if x != 150 || y != -250 {
		t.Fatalf("point = (%d,%d), want (150,-250)", x, y)
	}
}

func TestEncodePrecisionAndOffset(t *testing.T) {
	opts := Options{Precision: 3, OffsetX: 100, OffsetY: 200}
	blob, err := Encode(geom.Point{100.001, 200.002}, opts)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(blob, opts)
	if err != nil {
		t.Fatal(err)
	}
	pt := back.(geom.Point)
	if math.Abs(pt[0]-100.001) > 0.0005 || math.Abs(pt[1]-200.002) > 0.0005 {
		t.Fatalf("got %v", pt)
	}
}

func TestEncodeRejectsOverflow(t *testing.T) {
	// 1e10 * 1 = 1e10 > MaxInt32 at precision 0... use huge coord.
	_, err := Encode(geom.Point{1e15, 0}, Options{Precision: 0})
	if err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestEncodeRejectsUnsupported(t *testing.T) {
	_, err := Encode(geom.Collection{}, Options{})
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
}

// geomsApproxEqual compares geometries within tolerance.
func geomsApproxEqual(a, b geom.Geometry, tol float64) bool {
	pa := flatten(a)
	pb := flatten(b)
	if len(pa) != len(pb) {
		return false
	}
	for i := range pa {
		if math.Abs(pa[i][0]-pb[i][0]) > tol || math.Abs(pa[i][1]-pb[i][1]) > tol {
			return false
		}
	}
	return true
}

func flatten(g geom.Geometry) [][2]float64 {
	var out [][2]float64
	switch t := g.(type) {
	case geom.Point:
		out = append(out, [2]float64(t))
	case geom.MultiPoint:
		for _, p := range t {
			out = append(out, [2]float64(p))
		}
	case geom.LineString:
		for _, p := range t {
			out = append(out, [2]float64(p))
		}
	case geom.MultiLineString:
		for _, l := range t {
			for _, p := range l {
				out = append(out, [2]float64(p))
			}
		}
	case geom.Polygon:
		for _, r := range t {
			for _, p := range r {
				out = append(out, [2]float64(p))
			}
		}
	case geom.MultiPolygon:
		for _, p := range t {
			for _, r := range p {
				for _, pt := range r {
					out = append(out, [2]float64(pt))
				}
			}
		}
	}
	return out
}
