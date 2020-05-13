package cmp

import (
	"sync/atomic"

	"github.com/go-spatial/geom"
)

// HiPrecision is a high precision for comparator
const HiPrecision = 0.000000001

// HiCMP is a high precision comparator
var HiCMP = New(HiPrecision)

// compare will have six digits of precision by default.
var compare = func() (av atomic.Value) {
	av.Store(NewForNumPrecision(6))
	return av
}()

// DefaultCompare returns the current default compare
func DefaultCompare() Compare {
	return compare.Load().(Compare)
}

// SetDefault will set the default for the package.
func SetDefault(cmp Compare) Compare {
	old := compare.Load().(Compare)
	compare.Store(cmp)
	return old
}

// Tolerances returns the default tolerance values
func Tolerances() (float64, int64) { return DefaultCompare().Tolerances() }

// Float compares two floats to see if they are within 0.00001 from each other. This is the best way to compare floats.
func Float(f1, f2 float64) bool { return DefaultCompare().Float(f1, f2) }

// FloatSlice compare two sets of float slices.
func FloatSlice(f1, f2 []float64) bool { return DefaultCompare().FloatSlice(f1, f2) }

// Extent will check to see if the Extents's are the same.
func Extent(extent1, extent2 [4]float64) bool { return DefaultCompare().Extent(extent1, extent2) }

// GeomExtent will check to see if geom.BoundingBox's are the same.
func GeomExtent(extent1, extent2 geom.Extenter) bool {
	return DefaultCompare().GeomExtent(extent1, extent2)
}

// PointLess returns weather p1 is < p2 by first comparing the X values, and if they are the same the Y values.
func PointLess(p1, p2 [2]float64) bool { return DefaultCompare().PointLess(p1, p2) }

// PointEqual returns weather both points have the same value for x,y.
func PointEqual(p1, p2 [2]float64) bool { return DefaultCompare().PointEqual(p1, p2) }

// GeomPointEqual returns weather both points have the same value for x,y.
func GeomPointEqual(p1, p2 geom.Point) bool { return DefaultCompare().GeomPointEqual(p1, p2) }

// MultiPointEqual will check to see see if the given slices are the same.
func MultiPointEqual(p1, p2 [][2]float64) bool { return DefaultCompare().MultiPointEqual(p1, p2) }

// LineStringEqual given two LineStrings it will check to see if the line
// strings have the same points in the same order.
func LineStringEqual(v1, v2 [][2]float64) bool { return DefaultCompare().LineStringEqual(v1, v2) }

// MultiLineEqual will return if the two multilines are equal.
func MultiLineEqual(ml1, ml2 [][][2]float64) bool { return DefaultCompare().MultiLineEqual(ml1, ml2) }

// PolygonEqual will return weather the two polygons are the same.
func PolygonEqual(ply1, ply2 [][][2]float64) bool { return DefaultCompare().PolygonEqual(ply1, ply2) }

// PointerEqual will check to see if the x and y of both points are the same.
func PointerEqual(geo1, geo2 geom.Pointer) bool { return DefaultCompare().PointerEqual(geo1, geo2) }

// PointerLess returns weather p1 is < p2 by first comparing the X values, and if they are the same the Y values.
func PointerLess(p1, p2 geom.Pointer) bool { return DefaultCompare().PointLess(p1.XY(), p2.XY()) }

// MultiPointerEqual will check to see if the provided multipoints have the same points.
func MultiPointerEqual(geo1, geo2 geom.MultiPointer) bool {
	return DefaultCompare().MultiPointerEqual(geo1, geo2)
}

// LineStringerEqual will check to see if the two linestrings passed to it are equal, if
// there lengths are both the same, and the sequence of points are in the same order.
// The points don't have to be in the same index point in both line strings.
func LineStringerEqual(geo1, geo2 geom.LineStringer) bool {
	return DefaultCompare().LineStringerEqual(geo1, geo2)
}

// MultiLineStringerEqual will check to see if the 2 MultiLineStrings pass to it
// are equal. This is done by converting them to lineStrings and using MultiLineEqual
func MultiLineStringerEqual(geo1, geo2 geom.MultiLineStringer) bool {
	return DefaultCompare().MultiLineStringerEqual(geo1, geo2)
}

// PolygonerEqual will check to see if the Polygoners are the same, by checking
// if the linearRings are equal.
func PolygonerEqual(geo1, geo2 geom.Polygoner) bool {
	return DefaultCompare().PolygonerEqual(geo1, geo2)
}

// MultiPolygonerEqual will check to see if the given multipolygoners are the same, by check each of the constitute
// polygons to see if they match.
func MultiPolygonerEqual(geo1, geo2 geom.MultiPolygoner) bool {
	return DefaultCompare().MultiPolygonerEqual(geo1, geo2)
}

// CollectionerEqual will check if the two collections are equal based on length
// then if each geometry inside is equal. Therefor order matters.
func CollectionerEqual(col1, col2 geom.Collectioner) bool {
	return DefaultCompare().CollectionerEqual(col1, col2)
}

// GeometryEqual checks if the two geometries are of the same type and then
// calls the type method to check if they are equal
func GeometryEqual(g1, g2 geom.Geometry) bool { return DefaultCompare().GeometryEqual(g1, g2) }
