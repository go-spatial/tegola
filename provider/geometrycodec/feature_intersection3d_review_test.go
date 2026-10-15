package geometrycodec_test

import (
	"errors"
	"math"
	"testing"

	"github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// Integer affine surfaces give an oracle independent of the production plane,
// projection and rational point-in-ring implementations.
func TestReview3DPlanarHolesGrid(t *testing.T) {
	frames := []struct {
		name  string
		point func(int, int) [3]float64
	}{
		{"tilted", func(x, y int) [3]float64 { return [3]float64{float64(x), float64(y), float64(2*x + 3*y + 5)} }},
		{"vertical", func(x, y int) [3]float64 { return [3]float64{5, float64(x), float64(y)} }},
	}
	for _, frame := range frames {
		t.Run(frame.name, func(t *testing.T) {
			p := geom.PolygonZ{
				{frame.point(0, 0), frame.point(10, 0), frame.point(10, 10), frame.point(0, 10)},
				{frame.point(3, 3), frame.point(7, 3), frame.point(7, 7), frame.point(3, 7)},
			}
			for _, reverse := range []bool{false, true} {
				if reverse {
					for _, ring := range p {
						for i, j := 0, len(ring)-1; i < j; i, j = i+1, j-1 {
							ring[i], ring[j] = ring[j], ring[i]
						}
					}
				}
				for x := -1; x <= 11; x++ {
					for y := -1; y <= 11; y++ {
						point := frame.point(x, y)
						box := [6]float64{point[0], point[1], point[2], point[0], point[1], point[2]}
						want := x >= 0 && x <= 10 && y >= 0 && y <= 10 && !(x > 3 && x < 7 && y > 3 && y < 7)
						got, err := codec.FeatureGeometryIntersectsExtent3D(p, &box)
						if err != nil || got != want {
							t.Fatalf("point (%d,%d),reverse%v: %v,%v want%v", x, y, reverse, got, err, want)
						}
						// Offset the coordinate normal to the surface; all values are exact integers.
						axis := 2
						if frame.name == "vertical" {
							axis = 0
						}
						box[axis]++
						box[axis+3]++
						if got, err := codec.FeatureGeometryIntersectsExtent3D(p, &box); err != nil || got {
							t.Fatalf("off-plane point accepted: %v %v", got, err)
						}
					}
				}
			}
		})
	}
}

func TestReview3DExactExtremeAndSubnormalSurfaces(t *testing.T) {
	max := math.MaxFloat64
	extreme := geom.PolygonZ{{{-max, -max, 0}, {max, -max, 0}, {max, max, 0}, {-max, max, 0}}}
	if got, err := codec.FeatureGeometryIntersectsExtent3D(extreme, &[6]float64{0, 0, 0, 0, 0, 0}); err != nil || !got {
		t.Fatalf("extreme surface: %v %v", got, err)
	}
	u := math.SmallestNonzeroFloat64
	tiny := geom.PolygonZ{
		{{0, 0, 0}, {6 * u, 0, 0}, {6 * u, 6 * u, 0}, {0, 6 * u, 0}},
		{{2 * u, 2 * u, 0}, {4 * u, 2 * u, 0}, {4 * u, 4 * u, 0}, {2 * u, 4 * u, 0}},
	}
	for _, tc := range []struct {
		x    float64
		want bool
	}{{u, true}, {2 * u, true}, {3 * u, false}, {4 * u, true}, {5 * u, true}, {7 * u, false}} {
		box := [6]float64{tc.x, 3 * u, 0, tc.x, 3 * u, 0}
		if got, err := codec.FeatureGeometryIntersectsExtent3D(tiny, &box); err != nil || got != tc.want {
			t.Fatalf("subnormal x%v: %v %v want%v", tc.x, got, err, tc.want)
		}
	}
}

func TestReview3DFullTreeValidationAndDetachedProjection(t *testing.T) {
	bad := geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0, 1, 0}}}
	g := geom.Collection{geom.Point{0, 0}, geom.Collection{codec.MultiPolygonZ{nil, bad}}}
	if _, err := codec.FeatureGeometryIntersectsExtent3D(g, &[6]float64{0, 0, 100, 0, 0, 101}); !errors.Is(err, codec.ErrUnsupportedFeatureSpatialGeometry) {
		t.Fatalf("matching XY hid nonplanar sibling: %v", err)
	}
	valid := geom.Collection{codec.MultiPolygonZ{{{{0, 0, 5}, {1, 0, 5}, {0, 1, 5}}}}, geom.MultiLineStringZ{{{0, 0, 2}, {1, 1, 3}}}}
	copy, err := codec.FeatureGeometryXYProjection(valid)
	if err != nil {
		t.Fatal(err)
	}
	copy.(geom.Collection)[0].(geom.MultiPolygon)[0][0][0][0] = 99
	copy.(geom.Collection)[1].(geom.MultiLineString)[0][0][0] = 98
	if valid[0].(codec.MultiPolygonZ)[0][0][0][0] != 0 || valid[1].(geom.MultiLineStringZ)[0][0][0] != 0 {
		t.Fatal("projection aliases recursive input")
	}
}
