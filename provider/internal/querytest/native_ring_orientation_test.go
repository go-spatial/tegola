package querytest

import (
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestNativeRingOrientationExact(t *testing.T) {
	ring := [][3]float64{{0, 0, 1}, {4, 0, 2}, {4, 4, 3}, {0, 4, 4}, {0, 0, 1}}
	reverse := [][3]float64{ring[0], ring[3], ring[2], ring[1], ring[4]}
	hole := [][3]float64{{1, 1, 5}, {2, 1, 6}, {2, 2, 7}, {1, 2, 8}, {1, 1, 5}}
	expected := geom.Collection{geometrycodec.MultiPolygonZ{{ring, hole}}, geom.LineStringZ{{1, 2, 3}, {4, 5, 6}}}
	before := geom.Collection{geometrycodec.MultiPolygonZ{{nativeCopy(ring), nativeCopy(hole)}}, geom.LineStringZ{{1, 2, 3}, {4, 5, 6}}}
	actual := geom.Collection{geometrycodec.MultiPolygonZ{{reverse, {hole[0], hole[3], hole[2], hole[1], hole[4]}}}, geom.LineStringZ{{1, 2, 3}, {4, 5, 6}}}
	normalized := normalizeNativeRingOrientation(expected)
	if !reflect.DeepEqual(normalized, normalizeNativeRingOrientation(actual)) {
		t.Fatal("exact winding reversal differs")
	}
	if !reflect.DeepEqual(expected, before) {
		t.Fatal("oracle mutated")
	}
	normalized.(geom.Collection)[0].(geometrycodec.MultiPolygonZ)[0][0][0][2] = 100
	if !reflect.DeepEqual(expected, before) {
		t.Fatal("normalized copy aliases oracle")
	}
	for _, closed := range []bool{false, true} {
		a, b := ring, reverse
		if !closed {
			a, b = a[:4], b[:4]
		}
		if !reflect.DeepEqual(nativeRing(a, nativeXYZLess), nativeRing(b, nativeXYZLess)) {
			t.Fatal("closure-preserving reversal differs")
		}
	}
	if reflect.DeepEqual(normalizeRingClosure(geom.PolygonZ{ring}), normalizeRingClosure(geom.PolygonZ{reverse})) {
		t.Fatal("raw default admits reversal")
	}
}

func TestNativeRingOrientationRejectsOtherChanges(t *testing.T) {
	ring := [][3]float64{{0, 0, 1}, {4, 0, 2}, {4, 4, 3}, {0, 4, 4}, {0, 0, 1}}
	hole := [][3]float64{{1, 1, 5}, {2, 1, 6}, {2, 2, 7}, {1, 2, 8}, {1, 1, 5}}
	original := geom.PolygonZ{ring, hole}
	changedZ := nativeCopy(ring)
	changedZ[1][2]++
	changedVertex := nativeCopy(ring)
	changedVertex[1][0] += 1e-12
	variants := map[string]geom.Geometry{
		"Z": geom.PolygonZ{changedZ, hole}, "vertex": geom.PolygonZ{changedVertex, hole},
		"hole missing": geom.PolygonZ{ring}, "ring order": geom.PolygonZ{hole, ring},
		"start":   geom.PolygonZ{{ring[1], ring[2], ring[3], ring[0], ring[1]}, hole},
		"closure": geom.PolygonZ{ring[:4], hole}, "extra vertex": geom.PolygonZ{append(nativeCopy(ring[:4]), ring[3], ring[4]), hole},
		"family": geometrycodec.MultiPolygonZ{{ring, hole}}, "nesting": geom.Collection{original},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			if reflect.DeepEqual(normalizeNativeRingOrientation(original), normalizeNativeRingOrientation(v)) {
				t.Fatal("non-winding change admitted")
			}
		})
	}
	line := geom.LineStringZ(ring)
	if reflect.DeepEqual(normalizeNativeRingOrientation(line), normalizeNativeRingOrientation(geom.LineStringZ{ring[0], ring[3], ring[2], ring[1], ring[4]})) {
		t.Fatal("line reversal admitted")
	}
	xy := geom.Polygon{{{0, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}}}
	xr := geom.Polygon{{{0, 0}, {0, 2}, {2, 2}, {2, 0}, {0, 0}}}
	if !reflect.DeepEqual(normalizeNativeRingOrientation(geom.MultiPolygon{xy}), normalizeNativeRingOrientation(geom.MultiPolygon{xr})) {
		t.Fatal("XY multipolygon reversal differs")
	}
}
