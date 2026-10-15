package geometrycodec_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestFeatureGeometryIntersectsExtent(t *testing.T) {
	box := geom.Extent{1, 1, 2, 2}
	polygon := geom.Polygon{
		{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
		{{3, 3}, {7, 3}, {7, 7}, {3, 7}},
	}
	cases := []struct {
		name string
		g    geom.Geometry
		box  geom.Extent
		want bool
	}{
		{name: "nil absence", box: box, want: true},
		{name: "point boundary", g: geom.Point{1, 1}, box: box, want: true},
		{name: "point outside", g: geom.Point{0, 0}, box: box},
		{name: "multipoint gap", g: geom.MultiPoint{{0, 0}, {3, 3}}, box: box},
		{name: "multipoint hit", g: geom.MultiPoint{{0, 0}, {1, 1}}, box: box, want: true},
		{name: "line crosses", g: geom.LineString{{0, 0}, {3, 3}}, box: box, want: true},
		{name: "line envelope gap", g: geom.LineString{{0, 0}, {3, 0}, {3, 3}}, box: box},
		{name: "collinear outside", g: geom.LineString{{0, 1}, {0.5, 1}}, box: box},
		{name: "line edge", g: geom.LineString{{0, 1}, {3, 1}}, box: box, want: true},
		{name: "multiline gap", g: geom.MultiLineString{{{0, 0}, {0, 3}}, {{3, 0}, {3, 3}}}, box: box},
		{name: "multiline hit", g: geom.MultiLineString{{{0, 0}, {0, 3}}, {{0, 1}, {3, 1}}}, box: box, want: true},
		{name: "inside polygon", g: polygon, box: box, want: true},
		{name: "inside polygon hole", g: polygon, box: geom.Extent{4, 4, 6, 6}},
		{name: "hole boundary included", g: polygon, box: geom.Extent{3, 4, 3, 5}, want: true},
		{name: "polygon in box", g: polygon, box: geom.Extent{-1, -1, 11, 11}, want: true},
		{name: "polygon outside", g: polygon, box: geom.Extent{11, 11, 12, 12}},
		{name: "multipolygon hole", g: geom.MultiPolygon{polygon}, box: geom.Extent{4, 4, 6, 6}},
		{name: "multipolygon hit", g: geom.MultiPolygon{polygon}, box: box, want: true},
		{name: "collection gap", g: geom.Collection{geom.Point{0, 0}, geom.Point{3, 3}}, box: box},
		{name: "nested hit", g: geom.Collection{geom.Collection{geom.Point{1, 1}}}, box: box, want: true},
		{name: "degenerate point crossing", g: geom.LineString{{0, 0}, {3, 3}}, box: geom.Extent{1, 1, 1, 1}, want: true},
		{name: "degenerate point gap", g: geom.LineString{{0, 0}, {3, 0}, {3, 3}}, box: geom.Extent{1, 1, 1, 1}},
		{name: "degenerate segment", g: geom.LineString{{0, 1}, {3, 1}}, box: geom.Extent{1, 0, 1, 2}, want: true},
		{name: "degenerate polygon hole point", g: polygon, box: geom.Extent{5, 5, 5, 5}},
		{name: "closed polygon accepted", g: geom.Polygon{{{0, 0}, {3, 0}, {0, 3}, {0, 0}}}, box: geom.Extent{0.5, 0.5, 1, 1}, want: true},
		{name: "unclosed polygon implicitly closed", g: geom.Polygon{{{0, 0}, {3, 0}, {0, 3}}}, box: geom.Extent{0.5, 0.5, 1, 1}, want: true},
		{name: "empty multipoint absence", g: geom.MultiPoint{}, box: box, want: true},
		{name: "empty line absence", g: geom.LineString{}, box: box, want: true},
		{name: "empty polygon absence", g: geom.Polygon{}, box: box, want: true},
		{name: "empty multiline absence", g: geom.MultiLineString{{}}, box: box, want: true},
		{name: "empty multipolygon absence", g: geom.MultiPolygon{{}}, box: box, want: true},
		{name: "empty nested collection absence", g: geom.Collection{nil, geom.Collection{geom.LineString{}}}, box: box, want: true},
		{name: "partial empty collection not absence", g: geom.Collection{nil, geom.Point{0, 0}}, box: box},
		{name: "partial empty multiline not absence", g: geom.MultiLineString{{}, {{0, 0}, {0, 3}}}, box: box},
		{name: "overflow orientation crossing", g: geom.LineString{{-math.MaxFloat64, -math.MaxFloat64}, {math.MaxFloat64, math.MaxFloat64}}, box: box, want: true},
		{name: "underflow orientation crossing", g: geom.LineString{{0, 0}, {1e-200, 1e-200}}, box: geom.Extent{5e-201, 5e-201, 5e-201, 5e-201}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.box
			got, err := codec.FeatureGeometryIntersectsExtent(tc.g, &tc.box)
			if err != nil || got != tc.want {
				t.Fatalf("intersection=%v, want=%v, err=%v", got, tc.want, err)
			}
			if tc.box != before {
				t.Fatal("extent mutated")
			}
		})
	}
}

func TestFeatureIntersectionRejectsMalformedGeometry(t *testing.T) {
	box := geom.Extent{0, 0, 2, 2}
	cases := []geom.Geometry{
		geom.Point{math.NaN(), 0}, geom.Point{0, math.Inf(1)},
		geom.PointZ{0, 0, 0}, geom.LineString{{0, 0}}, geom.Polygon{{{0, 0}, {1, 1}}},
		geom.Polygon{{{0, 0}, {1, 0}, {1, 1}}, {}},
		geom.Collection{geom.Point{1, 1}, geom.Point{math.NaN(), 0}},
		geom.MultiLineString{{{0, 0}, {1, 1}}, {{math.Inf(-1), 0}, {1, 1}}},
	}
	for _, g := range cases {
		for _, extent := range []*geom.Extent{&box, nil} {
			if hit, err := codec.FeatureGeometryIntersectsExtent(g, extent); err == nil || hit {
				t.Fatalf("malformed geometry accepted: %T, hit=%v, err=%v", g, hit, err)
			}
		}
	}
	for _, extent := range []geom.Extent{{2, 0, 1, 1}, {0, 2, 1, 1}, {math.NaN(), 0, 1, 1}} {
		if hit, err := codec.FeatureGeometryIntersectsExtent(nil, &extent); err == nil || hit {
			t.Fatalf("invalid extent accepted: %v", extent)
		}
	}
}

func TestFeatureIntersectionDoesNotMutateGeometry(t *testing.T) {
	g := geom.Polygon{{{0, 0}, {4, 0}, {4, 4}, {0, 4}}, {{1, 1}, {2, 1}, {2, 2}, {1, 2}}}
	before := geom.Polygon{{{0, 0}, {4, 0}, {4, 4}, {0, 4}}, {{1, 1}, {2, 1}, {2, 2}, {1, 2}}}
	if _, err := codec.FeatureGeometryIntersectsExtent(g, &geom.Extent{1.1, 1.1, 1.9, 1.9}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, before) {
		t.Fatal("geometry coordinates changed")
	}
	if hit, err := codec.FeatureGeometryIntersectsExtent(g, nil); err != nil || !hit {
		t.Fatalf("nil extent restricts valid geometry: hit=%v, err=%v", hit, err)
	}
}
