package geometrycodec

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/alexeydott/geom"
)

// ErrUnsupportedFeatureSpatialGeometry identifies a geometry outside the raw
// feature profile, including nonplanar XYZ polygon surfaces.
var ErrUnsupportedFeatureSpatialGeometry = errors.New("unsupported feature spatial geometry")

// ValidateFeatureSpatialGeometry validates every child before selection. XYZ
// polygons are planar surfaces; measures and unrecognized types are unsupported.
func ValidateFeatureSpatialGeometry(g geom.Geometry) error {
	switch v := g.(type) {
	case geom.PointZ:
		return validateSpatialPoints([][3]float64{v})
	case geom.MultiPointZ:
		return validateSpatialPoints(v)
	case geom.LineStringZ:
		if len(v) == 1 {
			return fmt.Errorf("feature line requires at least two coordinates")
		}
		return validateSpatialPoints(v)
	case geom.MultiLineStringZ:
		for _, line := range v {
			if err := ValidateFeatureSpatialGeometry(geom.LineStringZ(line)); err != nil {
				return err
			}
		}
	case geom.PolygonZ:
		if len(v) == 0 {
			return nil
		}
		for _, ring := range v {
			if len(ring) < 3 {
				return fmt.Errorf("feature polygon ring requires at least three coordinates")
			}
			if err := validateSpatialPoints(ring); err != nil {
				return err
			}
		}
		_, err := spatialPolygonPlane(v)
		return err
	case MultiPolygonZ:
		for _, polygon := range v {
			if err := ValidateFeatureSpatialGeometry(geom.PolygonZ(polygon)); err != nil {
				return err
			}
		}
	case geom.Collection:
		for _, child := range v {
			if err := ValidateFeatureSpatialGeometry(child); err != nil {
				return err
			}
		}
	default:
		if err := validateFeatureGeometry(g); err != nil {
			// Preserve malformed XY errors; explicitly identify unsupported types.
			switch g.(type) {
			case nil, geom.Point, geom.MultiPoint, geom.LineString, geom.MultiLineString, geom.Polygon, geom.MultiPolygon:
				return err
			default:
				return fmt.Errorf("%w: %T", ErrUnsupportedFeatureSpatialGeometry, g)
			}
		}
	}
	return nil
}

func validateSpatialPoints(points [][3]float64) error {
	for _, p := range points {
		for _, x := range p {
			if !featureFinite(x) {
				return fmt.Errorf("feature geometry contains nonfinite coordinates")
			}
		}
	}
	return nil
}

// FeatureGeometryXYProjection returns an independently owned XY projection,
// validating XYZ ordinates and surfaces without altering the original geometry.
func FeatureGeometryXYProjection(g geom.Geometry) (geom.Geometry, error) {
	if err := ValidateFeatureSpatialGeometry(g); err != nil {
		return nil, err
	}
	return spatialXYCopy(g), nil
}

func spatialXYCopy(g geom.Geometry) geom.Geometry {
	points := func(v [][3]float64) [][2]float64 {
		out := make([][2]float64, len(v))
		for i, p := range v {
			out[i] = [2]float64{p[0], p[1]}
		}
		return out
	}
	switch v := g.(type) {
	case nil:
		return nil
	case geom.Point:
		return v
	case geom.PointZ:
		return geom.Point{v[0], v[1]}
	case geom.MultiPoint:
		return append(geom.MultiPoint(nil), v...)
	case geom.MultiPointZ:
		return geom.MultiPoint(points(v))
	case geom.LineString:
		return append(geom.LineString(nil), v...)
	case geom.LineStringZ:
		return geom.LineString(points(v))
	case geom.MultiLineString:
		out := make(geom.MultiLineString, len(v))
		for i, line := range v {
			out[i] = append([][2]float64(nil), line...)
		}
		return out
	case geom.MultiLineStringZ:
		out := make(geom.MultiLineString, len(v))
		for i, line := range v {
			out[i] = points(line)
		}
		return out
	case geom.Polygon:
		out := make(geom.Polygon, len(v))
		for i, ring := range v {
			out[i] = append([][2]float64(nil), ring...)
		}
		return out
	case geom.PolygonZ:
		out := make(geom.Polygon, len(v))
		for i, ring := range v {
			out[i] = points(ring)
		}
		return out
	case geom.MultiPolygon:
		out := make(geom.MultiPolygon, len(v))
		for i, polygon := range v {
			out[i] = spatialXYCopy(geom.Polygon(polygon)).(geom.Polygon)
		}
		return out
	case MultiPolygonZ:
		out := make(geom.MultiPolygon, len(v))
		for i, polygon := range v {
			out[i] = spatialXYCopy(geom.PolygonZ(polygon)).(geom.Polygon)
		}
		return out
	case geom.Collection:
		out := make(geom.Collection, len(v))
		for i, child := range v {
			out[i] = spatialXYCopy(child)
		}
		return out
	}
	return nil // Validation excludes other types.
}

// FeatureGeometryIntersectsExtent3D tests inclusive boxes in the geometry's
// coordinate frame. XY children have an unconstrained vertical dimension. Nil
// and wholly empty geometry match; empty children do not override populated ones.
// Rational arithmetic evaluates the exact supplied IEEE-754 coordinates, without
// epsilon tolerances. A nil box validates without imposing a spatial restriction.
func FeatureGeometryIntersectsExtent3D(g geom.Geometry, bounds *[6]float64) (bool, error) {
	if bounds != nil {
		for i, x := range bounds {
			if !featureFinite(x) {
				return false, fmt.Errorf("feature bounds contain nonfinite coordinates")
			}
			if i < 3 && x > bounds[i+3] {
				return false, fmt.Errorf("feature bounds minimum exceeds maximum")
			}
		}
	}
	if err := ValidateFeatureSpatialGeometry(g); err != nil {
		return false, err
	}
	if bounds == nil || spatialEmpty(g) {
		return true, nil
	}
	return spatialIntersects(g, bounds), nil
}

func spatialEmpty(g geom.Geometry) bool {
	switch v := g.(type) {
	case geom.MultiPointZ:
		return len(v) == 0
	case geom.LineStringZ:
		return len(v) == 0
	case geom.MultiLineStringZ:
		for _, line := range v {
			if len(line) != 0 {
				return false
			}
		}
		return true
	case geom.PolygonZ:
		return len(v) == 0
	case MultiPolygonZ:
		for _, polygon := range v {
			if len(polygon) != 0 {
				return false
			}
		}
		return true
	case geom.Collection:
		for _, child := range v {
			if !spatialEmpty(child) {
				return false
			}
		}
		return true
	case geom.PointZ:
		return false
	default:
		return featureGeometryEmpty(g)
	}
}

func spatialIntersects(g geom.Geometry, box *[6]float64) bool {
	switch v := g.(type) {
	case geom.PointZ:
		return spatialSegmentBox(v, v, box)
	case geom.MultiPointZ:
		for _, p := range v {
			if spatialSegmentBox(p, p, box) {
				return true
			}
		}
	case geom.LineStringZ:
		for i := 1; i < len(v); i++ {
			if spatialSegmentBox(v[i-1], v[i], box) {
				return true
			}
		}
	case geom.MultiLineStringZ:
		for _, line := range v {
			if len(line) != 0 && spatialIntersects(geom.LineStringZ(line), box) {
				return true
			}
		}
	case geom.PolygonZ:
		return spatialPolygonBox(v, box)
	case MultiPolygonZ:
		for _, polygon := range v {
			if len(polygon) != 0 && spatialPolygonBox(geom.PolygonZ(polygon), box) {
				return true
			}
		}
	case geom.Collection:
		for _, child := range v {
			if !spatialEmpty(child) && spatialIntersects(child, box) {
				return true
			}
		}
	default:
		e := geom.Extent{box[0], box[1], box[3], box[4]}
		return featureIntersects(g, &e)
	}
	return false
}

func spatialRat(x float64) *big.Rat     { return new(big.Rat).SetFloat64(x) }
func spatialSub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func spatialMul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }

type spatialVector [3]*big.Rat

func spatialVectorOf(p [3]float64) spatialVector {
	return spatialVector{spatialRat(p[0]), spatialRat(p[1]), spatialRat(p[2])}
}

func spatialDifference(a, b spatialVector) spatialVector {
	return spatialVector{spatialSub(a[0], b[0]), spatialSub(a[1], b[1]), spatialSub(a[2], b[2])}
}

func spatialDot(a, b spatialVector) *big.Rat {
	s := new(big.Rat)
	for i := range a {
		s.Add(s, spatialMul(a[i], b[i]))
	}
	return s
}

func spatialSegmentBox(a, b [3]float64, box *[6]float64) bool {
	lo, hi := new(big.Rat), big.NewRat(1, 1)
	for i := 0; i < 3; i++ {
		x, d := spatialRat(a[i]), spatialSub(spatialRat(b[i]), spatialRat(a[i]))
		min, max := spatialRat(box[i]), spatialRat(box[i+3])
		if d.Sign() == 0 {
			if x.Cmp(min) < 0 || x.Cmp(max) > 0 {
				return false
			}
			continue
		}
		u := new(big.Rat).Quo(spatialSub(min, x), d)
		v := new(big.Rat).Quo(spatialSub(max, x), d)
		if u.Cmp(v) > 0 {
			u, v = v, u
		}
		if u.Cmp(lo) > 0 {
			lo.Set(u)
		}
		if v.Cmp(hi) < 0 {
			hi.Set(v)
		}
		if lo.Cmp(hi) > 0 {
			return false
		}
	}
	return true
}

type spatialPlane struct {
	origin, normal spatialVector
	drop           int
}

func spatialPolygonPlane(p geom.PolygonZ) (spatialPlane, error) {
	origin := spatialVectorOf(p[0][0])
	var normal spatialVector
	var direction spatialVector
	found := false
	for i := 1; i < len(p[0]) && !found; i++ {
		b := spatialDifference(spatialVectorOf(p[0][i]), origin)
		if direction[0] == nil {
			if b[0].Sign() != 0 || b[1].Sign() != 0 || b[2].Sign() != 0 {
				direction = b
			}
			continue
		}
		for k := 0; k < 3; k++ {
			normal[k] = spatialSub(
				spatialMul(direction[(k+1)%3], b[(k+2)%3]),
				spatialMul(direction[(k+2)%3], b[(k+1)%3]),
			)
		}
		found = normal[0].Sign() != 0 || normal[1].Sign() != 0 || normal[2].Sign() != 0
	}
	if !found {
		return spatialPlane{}, fmt.Errorf("%w: degenerate XYZ polygon plane", ErrUnsupportedFeatureSpatialGeometry)
	}
	drop := 0
	for normal[drop].Sign() == 0 {
		drop++
	}
	for _, ring := range p {
		for _, point := range ring {
			if spatialDot(normal, spatialDifference(spatialVectorOf(point), origin)).Sign() != 0 {
				return spatialPlane{}, fmt.Errorf("%w: nonplanar XYZ polygon", ErrUnsupportedFeatureSpatialGeometry)
			}
		}
	}
	return spatialPlane{origin: origin, normal: normal, drop: drop}, nil
}

// Polygon/box contact occurs on a polygon edge, or a box edge crossing the
// polygon surface. This also covers containment and zero-width boxes.
func spatialPolygonBox(p geom.PolygonZ, box *[6]float64) bool {
	for _, ring := range p {
		for i, a := range ring {
			if spatialSegmentBox(a, ring[(i+1)%len(ring)], box) {
				return true
			}
		}
	}
	plane, _ := spatialPolygonPlane(p) // Already validated before matching.
	for mask := 0; mask < 8; mask++ {
		var a [3]float64
		for k := 0; k < 3; k++ {
			a[k] = box[k+3*((mask>>k)&1)]
		}
		for k := 0; k < 3; k++ {
			if mask&(1<<k) != 0 {
				continue
			}
			b := a
			b[k] = box[k+3]
			av, bv := spatialVectorOf(a), spatialVectorOf(b)
			fa := spatialDot(plane.normal, spatialDifference(av, plane.origin))
			fb := spatialDot(plane.normal, spatialDifference(bv, plane.origin))
			if fa.Sign() == 0 && spatialPointPolygon(av, p, plane.drop) {
				return true
			}
			if fb.Sign() == 0 && spatialPointPolygon(bv, p, plane.drop) {
				return true
			}
			if fa.Sign()*fb.Sign() >= 0 {
				continue
			}
			t := new(big.Rat).Quo(fa, spatialSub(fa, fb))
			var point spatialVector
			for j := range point {
				point[j] = new(big.Rat).Add(av[j], spatialMul(t, spatialSub(bv[j], av[j])))
			}
			if spatialPointPolygon(point, p, plane.drop) {
				return true
			}
		}
	}
	return false
}

func spatialPointPolygon(point spatialVector, p geom.PolygonZ, drop int) bool {
	inside, boundary := spatialPointRing(point, p[0], drop)
	if boundary {
		return true
	}
	if !inside {
		return false
	}
	for _, ring := range p[1:] {
		inside, boundary = spatialPointRing(point, ring, drop)
		if boundary {
			return true
		}
		if inside {
			return false
		}
	}
	return true
}

func spatialPointRing(point spatialVector, ring [][3]float64, drop int) (inside, boundary bool) {
	x, y := (drop+1)%3, (drop+2)%3
	for i, vertex := range ring {
		a, b := spatialVectorOf(vertex), spatialVectorOf(ring[(i+1)%len(ring)])
		cross := spatialSub(spatialMul(spatialSub(b[x], a[x]), spatialSub(point[y], a[y])), spatialMul(spatialSub(b[y], a[y]), spatialSub(point[x], a[x])))
		between := func(q, u, v *big.Rat) bool { return q.Cmp(u)*q.Cmp(v) <= 0 }
		if cross.Sign() == 0 && between(point[x], a[x], b[x]) && between(point[y], a[y], b[y]) {
			return false, true
		}
		if (a[y].Cmp(point[y]) > 0) != (b[y].Cmp(point[y]) > 0) {
			if cross.Sign()*spatialSub(b[y], a[y]).Sign() > 0 {
				inside = !inside
			}
		}
	}
	return inside, false
}
