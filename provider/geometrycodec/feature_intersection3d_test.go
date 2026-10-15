package geometrycodec

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
)

func TestFeatureIntersection3D(t *testing.T) {
	flat := geom.PolygonZ{{{0, 0, 5}, {10, 0, 5}, {10, 10, 5}, {0, 10, 5}}, {{3, 3, 5}, {7, 3, 5}, {7, 7, 5}, {3, 7, 5}}}
	vertical := geom.PolygonZ{{{5, 0, 0}, {5, 10, 0}, {5, 10, 10}, {5, 0, 10}}}
	tilted := geom.PolygonZ{{{0, 0, 0}, {10, 0, 10}, {10, 10, 20}, {0, 10, 10}}}
	tests := []struct {
		name string
		g    geom.Geometry
		box  [6]float64
		want bool
	}{
		{"point boundary", geom.PointZ{1, 2, 3}, [6]float64{1, 2, 3, 1, 2, 3}, true},
		{"point altitude misses", geom.PointZ{1, 2, 3}, [6]float64{0, 0, 4, 2, 3, 5}, false},
		{"segment correlation misses", geom.LineStringZ{{0, 0, 0}, {10, 0, 10}}, [6]float64{0, -1, 8, 2, 1, 10}, false},
		{"segment correlation touches", geom.LineStringZ{{0, 0, 0}, {10, 0, 10}}, [6]float64{0, -1, 2, 2, 1, 10}, true},
		{"surface containment", flat, [6]float64{1, 1, 4, 2, 2, 6}, true},
		{"surface hole", flat, [6]float64{4, 4, 4, 6, 6, 6}, false},
		{"surface hole boundary", flat, [6]float64{3, 4, 5, 3, 6, 5}, true},
		{"surface vertical misses", flat, [6]float64{1, 1, 6, 2, 2, 7}, false},
		{"vertical surface", vertical, [6]float64{4, 2, 2, 6, 3, 3}, true},
		{"vertical coplanar box", vertical, [6]float64{5, 2, 2, 5, 3, 3}, true},
		{"vertical degenerate point", vertical, [6]float64{5, 2, 2, 5, 2, 2}, true},
		{"tilted surface", tilted, [6]float64{2, 2, 4, 2, 2, 4}, true},
		{"tilted altitude false positive", tilted, [6]float64{2, 2, 8, 3, 3, 9}, false},
		{"multipolygon retains union", MultiPolygonZ{flat, vertical}, [6]float64{5, 2, 2, 5, 3, 3}, true},
		{"multipolygon empty sibling", MultiPolygonZ{nil, flat}, [6]float64{4, 4, 4, 6, 6, 6}, false},
		{"multiline empty sibling", geom.MultiLineStringZ{nil, {{4, 4, 4}, {5, 5, 5}}}, [6]float64{0, 0, 0, 1, 1, 1}, false},
		{"multipoint misses", geom.MultiPointZ{{0, 0, 2}, {1, 1, 3}}, [6]float64{0, 0, 0, 1, 1, 1}, false},
		{"XY unconstrained", geom.Point{1, 2}, [6]float64{0, 0, 100, 2, 3, 101}, true},
		{"empty collection", geom.Collection{nil, geom.LineStringZ{}}, [6]float64{0, 0, 0, 1, 1, 1}, true},
		{"empty child cannot match", geom.Collection{nil, geom.PointZ{4, 4, 4}}, [6]float64{0, 0, 0, 1, 1, 1}, false},
		{"mixed child union", geom.Collection{geom.PointZ{4, 4, 4}, geom.Point{0, 0}}, [6]float64{0, 0, 100, 1, 1, 101}, true},
		{"huge exact segment", geom.LineStringZ{{-math.MaxFloat64, 0, 0}, {math.MaxFloat64, 0, 0}}, [6]float64{0, 0, 0, 0, 0, 0}, true},
		{"subnormal segment", geom.LineStringZ{{0, 0, 0}, {math.SmallestNonzeroFloat64, 0, math.SmallestNonzeroFloat64}}, [6]float64{math.SmallestNonzeroFloat64, 0, 0, math.SmallestNonzeroFloat64, 0, 0}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FeatureGeometryIntersectsExtent3D(tc.g, &tc.box)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestFeatureSpatialValidation(t *testing.T) {
	bad := geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0, 1, 0}}}
	for _, g := range []geom.Geometry{bad, geom.Collection{geom.Point{0, 0}, bad}} {
		_, err := FeatureGeometryIntersectsExtent3D(g, &[6]float64{-1, -1, -1, 2, 2, 2})
		if !errors.Is(err, ErrUnsupportedFeatureSpatialGeometry) {
			t.Fatalf("expected unsupported nonplanarity: %v", err)
		}
	}
	for _, g := range []geom.Geometry{geom.PointM{0, 0, 1}, geom.PointZM{0, 0, 1, 2}, MultiPolygonZ{nil, bad}} {
		if err := ValidateFeatureSpatialGeometry(g); !errors.Is(err, ErrUnsupportedFeatureSpatialGeometry) {
			t.Fatalf("expected unsupported geometry for %T: %v", g, err)
		}
	}
	for _, g := range []geom.Geometry{
		geom.Collection{geom.Point{0, 0}, geom.PointZ{0, 0, math.Inf(1)}},
		geom.LineStringZ{{0, 0, 0}},
		geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}}},
	} {
		if _, err := FeatureGeometryIntersectsExtent3D(g, nil); err == nil {
			t.Fatalf("accepted malformed %T", g)
		}
	}
	for _, b := range [][6]float64{{0, 0, 1, 1, 1, 0}, {0, 0, 0, 1, 1, math.NaN()}} {
		if _, err := FeatureGeometryIntersectsExtent3D(nil, &b); err == nil {
			t.Fatal("accepted malformed bounds")
		}
	}
}

func TestFeatureSpatialProjectionOwnership(t *testing.T) {
	original := geom.Collection{geom.PolygonZ{{{0, 0, 5}, {1, 0, 5}, {0, 1, 5}}}, geom.MultiPolygon{{{{0, 0}, {1, 0}, {0, 1}}}}}
	want := geom.Collection{geom.Polygon{{{0, 0}, {1, 0}, {0, 1}}}, geom.MultiPolygon{{{{0, 0}, {1, 0}, {0, 1}}}}}
	got, err := FeatureGeometryXYProjection(original)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("projection %v, %v", got, err)
	}
	got.(geom.Collection)[0].(geom.Polygon)[0][0][0] = 77
	got.(geom.Collection)[1].(geom.MultiPolygon)[0][0][0][0] = 88
	if original[0].(geom.PolygonZ)[0][0][0] != 0 || original[1].(geom.MultiPolygon)[0][0][0][0] != 0 {
		t.Fatal("projection aliases input")
	}
}

func TestFeatureSpatialLongCollinearPrefix(t *testing.T) {
	// Exercise plane discovery on a large degenerate ring and an equally long
	// prefix before the first independent vector, without timing assumptions.
	ring := make([][3]float64, 12000)
	for i := range ring {
		x := float64(i / 2) // Repeated vertices also exercise zero vectors.
		ring[i] = [3]float64{x, 0, x}
	}
	if err := ValidateFeatureSpatialGeometry(geom.PolygonZ{ring}); !errors.Is(err, ErrUnsupportedFeatureSpatialGeometry) {
		t.Fatalf("collinear surface should be unsupported: %v", err)
	}
	ring = append(ring, [3]float64{6000, 1, 6000}, [3]float64{0, 1, 0})
	if err := ValidateFeatureSpatialGeometry(geom.PolygonZ{ring}); err != nil {
		t.Fatalf("long prefix should retain exact planar support: %v", err)
	}
}
