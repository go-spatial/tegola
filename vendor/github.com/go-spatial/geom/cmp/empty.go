package cmp

import (
	"github.com/go-spatial/geom"
)

func IsEmptyPoint(pt [2]float64) bool {
	return pt != pt
}

func IsEmptyPoints(pts [][2]float64) bool {
	for _, v := range pts {
		if !IsEmptyPoint(v) {
			return false
		}
	}

	return true
}

func IsEmptyLines(lns [][][2]float64) bool {
	for _, v := range lns {
		if !IsEmptyPoints(v) {
			return false
		}
	}

	return true
}

func IsEmptyGeo(geo geom.Geometry) (isEmpty bool) {
	if geom.IsNil(geo) {
		return true
	}

	switch g := geo.(type) {
	case [2]float64:
		return IsEmptyPoint(g)

	case geom.Point:
		return IsEmptyPoint(g.XY())

	case *geom.Point:
		if g == nil {
			return true
		}

		return IsEmptyPoint(g.XY())

	case [][2]float64:
		return IsEmptyPoints(g)

	case geom.MultiPoint:
		return IsEmptyPoints(g.Points())

	case *geom.MultiPoint:
		if g == nil {
			return true
		}

		return IsEmptyPoints(g.Points())

	case geom.LineString:
		return IsEmptyPoints(g.Vertices())

	case *geom.LineString:
		if g == nil {
			return true
		}
		return IsEmptyPoints(g.Vertices())

	case geom.MultiLineString:
		return IsEmptyLines(g.LineStrings())

	case *geom.MultiLineString:
		if g == nil {
			return true
		}
		return IsEmptyLines(g.LineStrings())

	case geom.Polygon:
		return IsEmptyLines(g.LinearRings())

	case *geom.Polygon:
		if g == nil {
			return true
		}
		return IsEmptyLines(g.LinearRings())

	case geom.MultiPolygon:
		for _, v := range g.Polygons() {
			if !IsEmptyLines(v) {
				return false
			}
		}

		return true

	case *geom.MultiPolygon:
		if g == nil {
			return true
		}

		for _, v := range g.Polygons() {
			if !IsEmptyLines(v) {
				return false
			}
		}

		return true

	case geom.Collection:
		// if one item in the geometries list is not empty
		// then the whole list is not empty
		for _, v := range g.Geometries() {
			if !IsEmptyGeo(v) {
				return false
			}
		}

		return true

	case *geom.Collection:
		if g == nil {
			return true
		}

		for _, v := range g.Geometries() {
			// if one item in the geometries list is not empty
			// then the whole list is not empty
			if !IsEmptyGeo(v) {
				return false
			}
		}

		return true

	default:
		return false
	}
}
