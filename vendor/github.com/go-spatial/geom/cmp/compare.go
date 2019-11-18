package cmp

import (
	"math"
	"sort"

	"github.com/go-spatial/geom"
)

// Compare holds the tolerances for the comparsion functions
type Compare struct {
	// Tolerance is the epsilon value used in comparing floats with zero
	Tolerance float64
	// BitTolerance is the epsilon value for comaparing float bit-patterns.
	BitTolerance int64
}

// New returns a new Compare object for the tolerance level, with a computed
// BitTolerance value
func New(tolerance float64) Compare {
	return Compare{
		Tolerance:    tolerance,
		BitTolerance: BitToleranceFor(tolerance),
	}
}

// NewForNumPrecision will return a comparator with the given number of precision digits
func NewForNumPrecision(prec int) Compare {
	tolerance := 1 / math.Pow10(prec)
	return New(tolerance)
}

// Tolerances returns the tolerances of the comparator
func (cmp Compare) Tolerances() (float64, int64) {
	return cmp.Tolerance, cmp.BitTolerance
}

// Float compares two floats to see if they are within the cmp tolerance of each other
func (cmp Compare) Float(f1, f2 float64) bool {
	tolerance, bitTolerance := cmp.Tolerances()
	// handle infinity
	if math.IsInf(f1, 0) || math.IsInf(f2, 0) {
		return math.IsInf(f1, -1) == math.IsInf(f2, -1) &&
			math.IsInf(f1, 1) == math.IsInf(f2, 1)
	}

	// -0.0 exist but -0.0 == 0.0 is true
	if f1 == 0 || f2 == 0 {
		return math.Abs(f2-f1) < tolerance
	}

	i1 := int64(math.Float64bits(f1))
	i2 := int64(math.Float64bits(f2))
	d := i2 - i1

	if d < 0 {
		return d > -bitTolerance
	}
	return d < bitTolerance
}

// FloatSlice compares two sets of float64 slices within the given tolerance.
func (cmp Compare) FloatSlice(f1, f2 []float64) bool {
	if len(f1) != len(f2) {
		return false
	}
	if len(f1) == 0 {
		return true
	}
	f1s := make([]float64, len(f1))
	f2s := make([]float64, len(f2))
	copy(f1s, f1)
	copy(f2s, f2)
	sort.Float64s(f1s)
	sort.Float64s(f2s)
	for i := range f1s {
		if !cmp.Float(f1s[i], f2s[i]) {
			return false
		}
	}
	return true
}

// Extent will check to see if the Extents's are the same.
func (cmp Compare) Extent(extent1, extent2 [4]float64) bool {
	return cmp.Float(extent1[0], extent2[0]) && cmp.Float(extent1[1], extent2[1]) &&
		cmp.Float(extent1[2], extent2[2]) && cmp.Float(extent1[3], extent2[3])
}

// GeomExtent will check to see if geom.BoundingBox's are the same.
func (cmp Compare) GeomExtent(extent1, extent2 geom.Extenter) bool {
	return cmp.Extent(extent1.Extent(), extent2.Extent())
}

// PointEqual returns weather both points have the same value for x,y.
func (cmp Compare) PointEqual(p1, p2 [2]float64) bool {
	return cmp.Float(p1[0], p2[0]) && cmp.Float(p1[1], p2[1])
}

// GeomPointEqual returns weather both points have the same value for x,y.
func (cmp Compare) GeomPointEqual(p1, p2 geom.Point) bool {
	return cmp.Float(p1[0], p2[0]) && cmp.Float(p1[1], p2[1])
}

// PointLess returns weather p1 is < p2 by first comparing the X values, and if they are the same the Y values.
func (cmp Compare) PointLess(p1, p2 [2]float64) bool {
	if p1[0] != p2[0] {
		return p1[0] < p2[0]
	}
	return p1[1] < p2[1]
}

// MultiPointEqual will check to see see if the given slices are the same.
func (cmp Compare) MultiPointEqual(p1, p2 [][2]float64) bool {
	if len(p1) != len(p2) {
		return false
	}
	// Need to make copies as sort.Sort mutates the slice.
	cv1 := make([][2]float64, len(p1))
	copy(cv1, p1)
	cv2 := make([][2]float64, len(p2))
	copy(cv2, p2)
	// We don't care about the order, just that it has the same of points.
	sort.Sort(ByXY(cv1))
	sort.Sort(ByXY(cv2))
	for i := range cv1 {
		if !cmp.PointEqual(cv1[i], cv2[i]) {
			return false
		}
	}
	return true
}

// LineStringEqual given two LineStrings it will check to see if the line
// strings have the same points in the same order.
func (cmp Compare) LineStringEqual(v1, v2 [][2]float64) bool {
	if len(v1) != len(v2) {
		return false
	}
	cv1 := make([][2]float64, len(v1))
	copy(cv1, v1)
	cv2 := make([][2]float64, len(v2))
	copy(cv2, v2)
	RotateToLeftMostPoint(cv1)
	RotateToLeftMostPoint(cv2)
	for i := range cv1 {
		if !cmp.PointEqual(cv1[i], cv2[i]) {
			return false
		}
	}
	return true
}

// MultiLineEqual will return if the two multilines are equal.
func (cmp Compare) MultiLineEqual(ml1, ml2 [][][2]float64) bool {
	if len(ml1) != len(ml2) {
		return false
	}
LOOP:
	for i := range ml1 {
		for j := range ml2 {
			if cmp.LineStringEqual(ml1[i], ml2[j]) {
				continue LOOP
			}
		}
		return false
	}
	return true
}

// PolygonEqual will return weather the two polygons are the same.
func (cmp Compare) PolygonEqual(ply1, ply2 [][][2]float64) bool {
	if len(ply1) != len(ply2) {
		return false
	}

	var points1, points2 [][2]float64
	for i := range ply1 {
		points1 = append(points1, ply1[i]...)
	}
	extent1 := geom.NewExtent(points1...)
	for i := range ply2 {
		points2 = append(points2, ply2[i]...)
	}
	extent2 := geom.NewExtent(points2...)
	if !GeomExtent(extent1, extent2) {
		return false
	}

	sort.Sort(bySubRingSizeXY(ply1))
	sort.Sort(bySubRingSizeXY(ply2))
	for i := range ply1 {
		if !cmp.LineStringEqual(ply1[i], ply2[i]) {
			return false
		}
	}
	return true
}

// PointerEqual will check to see if the x and y of both points are the same.
func (cmp Compare) PointerEqual(geo1, geo2 geom.Pointer) bool {
	if geo1Nil, geo2Nil := geo1 == NilPoint, geo2 == NilPoint; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return cmp.PointEqual(geo1.XY(), geo2.XY())
}

// PointerLess returns weather p1 is < p2 by first comparing the X values, and if they are the same the Y values.
func (cmp Compare) PointerLess(p1, p2 geom.Pointer) bool { return cmp.PointLess(p1.XY(), p2.XY()) }

// MultiPointerEqual will check to see if the provided multipoints have the same points.
func (cmp Compare) MultiPointerEqual(geo1, geo2 geom.MultiPointer) bool {
	if geo1Nil, geo2Nil := geo1 == NilMultiPoint, geo2 == NilMultiPoint; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return cmp.MultiPointEqual(geo1.Points(), geo2.Points())
}

// LineStringerEqual will check to see if the two linestrings passed to it are equal, if
// there lengths are both the same, and the sequence of points are in the same order.
// The points don't have to be in the same index point in both line strings.
func (cmp Compare) LineStringerEqual(geo1, geo2 geom.LineStringer) bool {
	if geo1Nil, geo2Nil := geo1 == NilLineString, geo2 == NilLineString; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return cmp.LineStringEqual(geo1.Vertices(), geo2.Vertices())
}

// MultiLineStringerEqual will check to see if the 2 MultiLineStrings pass to it
// are equal. This is done by converting them to lineStrings and using MultiLineEqual
func (cmp Compare) MultiLineStringerEqual(geo1, geo2 geom.MultiLineStringer) bool {
	// Polygon and MultiLine Strings are the same at this level.
	if geo1Nil, geo2Nil := geo1 == NilMultiLineString, geo2 == NilMultiLineString; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return cmp.MultiLineEqual(geo1.LineStrings(), geo2.LineStrings())
}

// PolygonerEqual will check to see if the Polygoners are the same, by checking
// if the linearRings are equal.
func (cmp Compare) PolygonerEqual(geo1, geo2 geom.Polygoner) bool {
	if geo1Nil, geo2Nil := geo1 == NilPoly, geo2 == NilPoly; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return cmp.PolygonEqual(geo1.LinearRings(), geo2.LinearRings())
}

// MultiPolygonerEqual will check to see if the given multipolygoners are the same, by check each of the constitute
// polygons to see if they match.
func (cmp Compare) MultiPolygonerEqual(geo1, geo2 geom.MultiPolygoner) bool {
	if geo1Nil, geo2Nil := geo1 == NilMultiPoly, geo2 == NilMultiPoly; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}

	p1, p2 := geo1.Polygons(), geo2.Polygons()
	if len(p1) != len(p2) {
		return false
	}
	sort.Sort(byPolygonMainSizeXY(p1))
	sort.Sort(byPolygonMainSizeXY(p2))
	for i := range p1 {
		if !cmp.PolygonEqual(p1[i], p2[i]) {
			return false
		}
	}
	return true
}

// CollectionerEqual will check if the two collections are equal based on length
// then if each geometry inside is equal. Therefor order matters.
func (cmp Compare) CollectionerEqual(col1, col2 geom.Collectioner) bool {
	if colNil, col2Nil := col1 == NilCollection, col2 == NilCollection; colNil || col2Nil {
		return colNil && col2Nil
	}

	g1, g2 := col1.Geometries(), col2.Geometries()
	if len(g1) != len(g2) {
		return false
	}
	for i := range g1 {
		if !cmp.GeometryEqual(g1[i], g2[i]) {
			return false
		}
	}
	return true
}

// GeometryEqual checks if the two geometries are of the same type and then
// calls the type method to check if they are equal
func (cmp Compare) GeometryEqual(g1, g2 geom.Geometry) bool {
	switch pg1 := g1.(type) {
	case geom.Pointer:
		if pg2, ok := g2.(geom.Pointer); ok {
			return cmp.PointerEqual(pg1, pg2)
		}
	case geom.MultiPointer:
		if pg2, ok := g2.(geom.MultiPointer); ok {
			return cmp.MultiPointerEqual(pg1, pg2)
		}
	case geom.LineStringer:
		if pg2, ok := g2.(geom.LineStringer); ok {
			return cmp.LineStringerEqual(pg1, pg2)
		}
	case geom.MultiLineStringer:
		if pg2, ok := g2.(geom.MultiLineStringer); ok {
			return cmp.MultiLineStringerEqual(pg1, pg2)
		}
	case geom.Polygoner:
		if pg2, ok := g2.(geom.Polygoner); ok {
			return cmp.PolygonerEqual(pg1, pg2)
		}
	case geom.MultiPolygoner:
		if pg2, ok := g2.(geom.MultiPolygoner); ok {
			return cmp.MultiPolygonerEqual(pg1, pg2)
		}
	case geom.Collectioner:
		if pg2, ok := g2.(geom.Collectioner); ok {
			return cmp.CollectionerEqual(pg1, pg2)
		}
	}
	return false
}
