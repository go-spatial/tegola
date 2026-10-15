package geometrycodec

import (
	"fmt"
	"math"
	"math/big"

	"github.com/alexeydott/geom"
)

// FeatureGeometryIntersectsExtent tests closed horizontal geometry against an
// inclusive extent, including degenerate extents. Nil and decoded empty geometry
// match as absent spatial geometry. A nil extent imposes no spatial restriction.
// All coordinates and children are validated before matching, even if another
// child matches. Inputs are never changed. This is separate from the tile path's
// conservative bounding-box helper and makes no CRS or vertical-coordinate claim.
// Rings may explicitly repeat their first coordinate or be implicitly closed,
// following geom.Polygon. Validation is structural, not a topology repair/check.
func FeatureGeometryIntersectsExtent(g geom.Geometry, e *geom.Extent) (bool, error) {
	if e != nil {
		for _, value := range e {
			if !featureFinite(value) {
				return false, fmt.Errorf("feature bounds contain nonfinite coordinates")
			}
		}
		if e[0] > e[2] || e[1] > e[3] {
			return false, fmt.Errorf("feature bounds minimum exceeds maximum")
		}
	}
	if err := validateFeatureGeometry(g); err != nil {
		return false, err
	}
	if e == nil {
		return true, nil
	}
	return featureIntersects(g, e), nil
}

func featureFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validateFeaturePoints(points [][2]float64) error {
	for _, point := range points {
		if !featureFinite(point[0]) || !featureFinite(point[1]) {
			return fmt.Errorf("feature geometry contains nonfinite coordinates")
		}
	}
	return nil
}

func validateFeatureGeometry(g geom.Geometry) error {
	switch value := g.(type) {
	case nil:
		return nil
	case geom.Point:
		return validateFeaturePoints([][2]float64{value})
	case geom.MultiPoint:
		return validateFeaturePoints(value)
	case geom.LineString:
		if len(value) == 1 {
			return fmt.Errorf("feature line requires at least two coordinates")
		}
		return validateFeaturePoints(value)
	case geom.MultiLineString:
		for _, line := range value {
			if err := validateFeatureGeometry(geom.LineString(line)); err != nil {
				return err
			}
		}
	case geom.Polygon:
		for _, ring := range value {
			if len(ring) < 3 {
				return fmt.Errorf("feature polygon ring requires at least three coordinates")
			}
			if err := validateFeaturePoints(ring); err != nil {
				return err
			}
		}
	case geom.MultiPolygon:
		for _, polygon := range value {
			if err := validateFeatureGeometry(geom.Polygon(polygon)); err != nil {
				return err
			}
		}
	case geom.Collection:
		for _, child := range value {
			if err := validateFeatureGeometry(child); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported horizontal feature geometry %T", g)
	}
	return nil
}

func featureIntersects(g geom.Geometry, e *geom.Extent) bool {
	if featureGeometryEmpty(g) {
		return true
	}
	switch value := g.(type) {
	case nil:
		return true
	case geom.Point:
		return e.ContainsPoint(value)
	case geom.MultiPoint:
		if len(value) == 0 {
			return true
		}
		for _, point := range value {
			if e.ContainsPoint(point) {
				return true
			}
		}
	case geom.LineString:
		return featureLineIntersects(value, e, false)
	case geom.MultiLineString:
		if len(value) == 0 {
			return true
		}
		for _, line := range value {
			if len(line) == 0 {
				continue
			}
			if featureLineIntersects(line, e, false) {
				return true
			}
		}
	case geom.Polygon:
		return featurePolygonIntersects(value, e)
	case geom.MultiPolygon:
		if len(value) == 0 {
			return true
		}
		for _, polygon := range value {
			if len(polygon) == 0 {
				continue
			}
			if featurePolygonIntersects(polygon, e) {
				return true
			}
		}
	case geom.Collection:
		if len(value) == 0 {
			return true
		}
		for _, child := range value {
			if featureGeometryEmpty(child) {
				continue
			}
			if featureIntersects(child, e) {
				return true
			}
		}
	}
	return false
}

func featureGeometryEmpty(g geom.Geometry) bool {
	switch value := g.(type) {
	case nil:
		return true
	case geom.MultiPoint:
		return len(value) == 0
	case geom.LineString:
		return len(value) == 0
	case geom.Polygon:
		return len(value) == 0
	case geom.MultiLineString:
		for _, line := range value {
			if len(line) != 0 {
				return false
			}
		}
		return true
	case geom.MultiPolygon:
		for _, polygon := range value {
			if len(polygon) != 0 {
				return false
			}
		}
		return true
	case geom.Collection:
		for _, child := range value {
			if !featureGeometryEmpty(child) {
				return false
			}
		}
		return true
	}
	return false
}

func featureExtentCorners(e *geom.Extent) [4][2]float64 {
	return [4][2]float64{{e[0], e[1]}, {e[2], e[1]}, {e[2], e[3]}, {e[0], e[3]}}
}

func featureLineIntersects(points [][2]float64, e *geom.Extent, closed bool) bool {
	if len(points) == 0 {
		return true
	}
	corners := featureExtentCorners(e)
	for i, point := range points {
		if e.ContainsPoint(point) {
			return true
		}
		if i == 0 && !closed {
			continue
		}
		previous := points[(i+len(points)-1)%len(points)]
		for j, corner := range corners {
			if featureSegmentsIntersect(previous, point, corner, corners[(j+1)%4]) {
				return true
			}
		}
	}
	return false
}

func featurePolygonIntersects(rings [][][2]float64, e *geom.Extent) bool {
	if len(rings) == 0 {
		return true
	}
	// Every outer/hole edge belongs to the polygon boundary.
	for _, ring := range rings {
		if featureLineIntersects(ring, e, true) {
			return true
		}
	}
	// With no crossing, a rectangle corner inside the outer ring and outside
	// every hole establishes overlap, including a rectangle inside the polygon.
	for _, corner := range featureExtentCorners(e) {
		if !featurePointInRing(corner, rings[0]) {
			continue
		}
		inHole := false
		for _, hole := range rings[1:] {
			if featurePointInRing(corner, hole) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

func featurePointInRing(point [2]float64, ring [][2]float64) bool {
	inside := false
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		if featureOrientation(a, b, point) == 0 && featurePointOnSegment(point, a, b) {
			return true
		}
		if (a[1] > point[1]) == (b[1] > point[1]) {
			continue
		}
		orientation := featureOrientation(a, b, point)
		if (orientation > 0) == (b[1] > a[1]) {
			inside = !inside
		}
	}
	return inside
}

func featurePointOnSegment(point, a, b [2]float64) bool {
	return point[0] >= math.Min(a[0], b[0]) && point[0] <= math.Max(a[0], b[0]) &&
		point[1] >= math.Min(a[1], b[1]) && point[1] <= math.Max(a[1], b[1])
}

func featureSegmentsIntersect(a, b, c, d [2]float64) bool {
	o1, o2 := featureOrientation(a, b, c), featureOrientation(a, b, d)
	o3, o4 := featureOrientation(c, d, a), featureOrientation(c, d, b)
	if o1 == 0 && featurePointOnSegment(c, a, b) || o2 == 0 && featurePointOnSegment(d, a, b) {
		return true
	}
	if o3 == 0 && featurePointOnSegment(a, c, d) || o4 == 0 && featurePointOnSegment(b, c, d) {
		return true
	}
	return o1*o2 < 0 && o3*o4 < 0
}

// featureOrientation uses a conservative error bound for a fast floating-point
// sign and exact rational arithmetic for degenerate, underflow or overflow cases.
// Rational conversion preserves the exact input IEEE-754 coordinate values.
func featureOrientation(a, b, c [2]float64) int {
	left := (a[0] - c[0]) * (b[1] - c[1])
	right := (a[1] - c[1]) * (b[0] - c[0])
	determinant := left - right
	errorBound := (math.Abs(left) + math.Abs(right)) * 1e-14
	// Subnormal products can lose relative precision before the subtraction;
	// do not accept a fast-path sign with an underflowed error bound.
	const smallestNormal = 0x1p-1022
	if featureFinite(determinant) && errorBound >= smallestNormal && math.Abs(determinant) > errorBound {
		if determinant > 0 {
			return 1
		}
		return -1
	}
	difference := func(x, y float64) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).SetFloat64(x), new(big.Rat).SetFloat64(y))
	}
	x := new(big.Rat).Mul(difference(a[0], c[0]), difference(b[1], c[1]))
	y := new(big.Rat).Mul(difference(a[1], c[1]), difference(b[0], c[0]))
	return x.Sub(x, y).Sign()
}
