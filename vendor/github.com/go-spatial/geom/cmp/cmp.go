package cmp

import (
	"math"
	"sort"

	"github.com/go-spatial/geom"
)

// TOLERANCE is the epsilon value used in comparing floats.
const TOLERANCE = 0.000001

var (
	NilPoint           = (*geom.Point)(nil)
	NilMultiPoint      = (*geom.MultiPoint)(nil)
	NilLineString      = (*geom.LineString)(nil)
	NilMultiLineString = (*geom.MultiLineString)(nil)
	NilPoly            = (*geom.Polygon)(nil)
	NilMultiPoly       = (*geom.MultiPolygon)(nil)
	NilCollection      = (*geom.Collection)(nil)
)

// FloatSlice compare two sets of float slices.
func FloatSlice(f1, f2 []float64) bool { return Float64Slice(f1, f2, TOLERANCE) }

// Float64Slice compares two sets of float64 slices within the given tolerance.
func Float64Slice(f1, f2 []float64, tolerance float64) bool {
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
		if !Float64(f1s[i], f2s[i], tolerance) {
			return false
		}
	}
	return true
}

// Float64 compares two floats to see if they are within the given tolerance.
func Float64(f1, f2, tolerance float64) bool {
	if math.IsInf(f1, 1) {
		return math.IsInf(f2, 1)
	}
	if math.IsInf(f2, 1) {
		return math.IsInf(f1, 1)
	}
	if math.IsInf(f1, -1) {
		return math.IsInf(f2, -1)
	}
	if math.IsInf(f2, -1) {
		return math.IsInf(f1, -1)
	}
	return math.Abs(f1-f2) < tolerance
}

// Float compares two floats to see if they are within 0.00001 from each other. This is the best way to compare floats.
func Float(f1, f2 float64) bool { return Float64(f1, f2, TOLERANCE) }

// Extent will check to see if the Extents's are the same.
func Extent(extent1, extent2 [4]float64) bool {
	return Float(extent1[0], extent2[0]) && Float(extent1[1], extent2[1]) &&
		Float(extent1[2], extent2[2]) && Float(extent1[3], extent2[3])
}

// GeomExtent will check to see if geom.BoundingBox's are the same.
func GeomExtent(extent1, extent2 geom.Extenter) bool {
	return Extent(extent1.Extent(), extent2.Extent())
}

// PointLess returns weather p1 is < p2 by first comparing the X values, and if they are the same the Y values.
func PointLess(p1, p2 [2]float64) bool {
	if p1[0] != p2[0] {
		return p1[0] < p2[0]
	}
	return p1[1] < p2[1]
}

// PointEqual returns weather both points have the same value for x,y.
func PointEqual(p1, p2 [2]float64) bool {
	return Float(p1[0], p2[0]) && Float(p1[1], p2[1])
}

// GeomPointEqual returns weather both points have the same value for x,y.
func GeomPointEqual(p1, p2 geom.Point) bool {
	return Float(p1[0], p2[0]) && Float(p1[1], p2[1])
}

// MultiPointEqual will check to see see if the given slices are the same.
func MultiPointEqual(p1, p2 [][2]float64) bool {
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
		if !PointEqual(cv1[i], cv2[i]) {
			return false
		}
	}
	return true
}

// LineStringEqual given two LineStrings it will check to see if the line
// strings have the same points in the same order.
func LineStringEqual(v1, v2 [][2]float64) bool {
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
		if !PointEqual(cv1[i], cv2[i]) {
			return false
		}
	}
	return true
}

// MultiLineEqual will return if the two multilines are equal.
func MultiLineEqual(ml1, ml2 [][][2]float64) bool {
	if len(ml1) != len(ml2) {
		return false
	}
LOOP:
	for i := range ml1 {
		for j := range ml2 {
			if LineStringEqual(ml1[i], ml2[j]) {
				continue LOOP
			}
		}
		return false
	}
	return true
}

// PolygonEqual will return weather the two polygons are the same.
func PolygonEqual(ply1, ply2 [][][2]float64) bool {
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
		if !LineStringEqual(ply1[i], ply2[i]) {
			return false
		}
	}
	return true
}

// PointerEqual will check to see if the x and y of both points are the same.
func PointerEqual(geo1, geo2 geom.Pointer) bool {
	if geo1Nil, geo2Nil := geo1 == NilPoint, geo2 == NilPoint; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return PointEqual(geo1.XY(), geo2.XY())
}

// PointerLess returns weather p1 is < p2 by first comparing the X values, and if they are the same the Y values.
func PointerLess(p1, p2 geom.Pointer) bool { return PointLess(p1.XY(), p2.XY()) }

// MultiPointerEqual will check to see if the provided multipoints have the same points.
func MultiPointerEqual(geo1, geo2 geom.MultiPointer) bool {
	if geo1Nil, geo2Nil := geo1 == NilMultiPoint, geo2 == NilMultiPoint; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return MultiPointEqual(geo1.Points(), geo2.Points())
}

// LineStringerEqual will check to see if the two linestrings passed to it are equal, if
// there lengths are both the same, and the sequence of points are in the same order.
// The points don't have to be in the same index point in both line strings.
func LineStringerEqual(geo1, geo2 geom.LineStringer) bool {
	if geo1Nil, geo2Nil := geo1 == NilLineString, geo2 == NilLineString; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return LineStringEqual(geo1.Verticies(), geo2.Verticies())
}

// MultiLineStringerEqual will check to see if the 2 MultiLineStrings pass to it
// are equal. This is done by converting them to lineStrings and using MultiLineEqual
func MultiLineStringerEqual(geo1, geo2 geom.MultiLineStringer) bool {
	// Polygon and MultiLine Strings are the same at this level.
	if geo1Nil, geo2Nil := geo1 == NilMultiLineString, geo2 == NilMultiLineString; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return MultiLineEqual(geo1.LineStrings(), geo2.LineStrings())
}

// PolygonerEqual will check to see if the Polygoners are the same, by checking
// if the linearRings are equal.
func PolygonerEqual(geo1, geo2 geom.Polygoner) bool {
	if geo1Nil, geo2Nil := geo1 == NilPoly, geo2 == NilPoly; geo1Nil || geo2Nil {
		return geo1Nil && geo2Nil
	}
	return PolygonEqual(geo1.LinearRings(), geo2.LinearRings())
}

// MultiPolygonerEqual will check to see if the given multipolygoners are the same, by check each of the constitute
// polygons to see if they match.
func MultiPolygonerEqual(geo1, geo2 geom.MultiPolygoner) bool {
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
		if !PolygonEqual(p1[i], p2[i]) {
			return false
		}
	}
	return true
}

// CollectionerEqual will check if the two collections are equal based on length
// then if each geometry inside is equal. Therefor order matters.
func CollectionerEqual(col1, col2 geom.Collectioner) bool {
	if colNil, col2Nil := col1 == NilCollection, col2 == NilCollection; colNil || col2Nil {
		return colNil && col2Nil
	}

	g1, g2 := col1.Geometries(), col2.Geometries()
	if len(g1) != len(g2) {
		return false
	}
	for i := range g1 {
		if !GeometryEqual(g1[i], g2[i]) {
			return false
		}
	}
	return true
}

// GeometryEqualChecks if the two geometries are of the same type and then
// calls the type method to check if they are equal
func GeometryEqual(g1, g2 geom.Geometry) bool {
	switch pg1 := g1.(type) {
	case geom.Pointer:
		if pg2, ok := g2.(geom.Pointer); ok {
			return PointerEqual(pg1, pg2)
		}
	case geom.MultiPointer:
		if pg2, ok := g2.(geom.MultiPointer); ok {
			return MultiPointerEqual(pg1, pg2)
		}
	case geom.LineStringer:
		if pg2, ok := g2.(geom.LineStringer); ok {
			return LineStringerEqual(pg1, pg2)
		}
	case geom.MultiLineStringer:
		if pg2, ok := g2.(geom.MultiLineStringer); ok {
			return MultiLineStringerEqual(pg1, pg2)
		}
	case geom.Polygoner:
		if pg2, ok := g2.(geom.Polygoner); ok {
			return PolygonerEqual(pg1, pg2)
		}
	case geom.MultiPolygoner:
		if pg2, ok := g2.(geom.MultiPolygoner); ok {
			return MultiPolygonerEqual(pg1, pg2)
		}
	case geom.Collectioner:
		if pg2, ok := g2.(geom.Collectioner); ok {
			return CollectionerEqual(pg1, pg2)
		}
	}
	return false
}
