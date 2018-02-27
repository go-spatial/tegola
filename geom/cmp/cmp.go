package cmp

import (
	"math"
	"sort"

	"github.com/go-spatial/tegola/geom"
)

const TOLERANCE = 0.000001

// Float64 compares two floats to see if they are within the given tolerance.
func Float64(f1, f2, tolerance float64) bool { return math.Abs(f1-f2) < tolerance }

// Float compares two floats to see if they are within 0.00001 from each other. This is the best way to compare floats.
func Float(f1, f2 float64) bool { return Float64(f1, f2, TOLERANCE) }

// BoundingBox will check to see if the BoundingBox's are the same.
func BoundingBox(bbox1, bbox2 [2][2]float64) bool {

	return Float(bbox1[0][0], bbox2[0][0]) && Float(bbox1[0][1], bbox2[0][1]) &&
		Float(bbox1[1][0], bbox2[1][0]) && Float(bbox1[1][1], bbox2[1][1])
}

func PointLess(p1, p2 [2]float64) bool {
	if p1[0] != p2[0] {
		return p1[0] < p2[0]
	}
	return p1[1] < p2[1]
}

func PointEqual(p1, p2 [2]float64) bool { return Float(p1[0], p2[0]) && Float(p1[1], p2[1]) }

// MultiPoint will check to see see if the given slices are the same.
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

// LineString given two LineStrings it will check to see if the line strings have the same
// points in the same order.
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

// Polygon will return weather the two polygons are the same.
func PolygonEqual(ply1, ply2 [][][2]float64) bool {
	if len(ply1) != len(ply2) {
		return false
	}

	var points1, points2 [][2]float64
	for i := range ply1 {
		points1 = append(points1, ply1[i]...)
	}
	bbox1 := geom.NewBBox(points1...)
	for i := range ply2 {
		points2 = append(points2, ply2[i]...)
	}
	bbox2 := geom.NewBBox(points2...)
	if !BoundingBox([2][2]float64(bbox1), [2][2]float64(bbox2)) {
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

// Point will check to see if the x and y of both points are the same.
func PointerEqual(geo1, geo2 geom.Pointer) bool { return PointEqual(geo1.XY(), geo2.XY()) }

// MultiPoint will check to see if the provided multipoints have the same points.
func MultiPointerEqual(geo1, geo2 geom.MultiPointer) bool {
	return MultiPointEqual(geo1.Points(), geo2.Points())
}

// LineString will check to see if the two linestrings passed to it are equal, if
// there lengths are both the same, and the sequence of points are in the same order.
// The points don't have to be in the same index point in both line strings.
func LineStringerEqual(geo1, geo2 geom.LineStringer) bool {
	return LineStringEqual(geo1.Verticies(), geo2.Verticies())
}

func MultiLineStringerEqual(geo1, geo2 geom.MultiLineStringer) bool {
	l1, l2 := geo1.LineStrings(), geo2.LineStrings()
	// Polygon and MultiLine Strings are the same at this level.
	return MultiLineEqual(l1, l2)
}

func PolygonerEqual(geo1, geo2 geom.Polygoner) bool {
	lr1, lr2 := geo1.LinearRings(), geo2.LinearRings()
	return PolygonEqual(lr1, lr2)
}

// MultiPolygoner will check to see if the given multipolygoners are the same, by check each of the constitute
// polygons to see if they match.
func MultiPolygonerEqual(geo1, geo2 geom.MultiPolygoner) bool {
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

func CollectionerEqual(col1, col2 geom.Collectioner) bool {
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
