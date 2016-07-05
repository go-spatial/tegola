package wkb

import (
	"fmt"
	"strings"
)

import "github.com/terranodo/tegola"

/*
This purpose of this file is to house the wkt functions. These functions are
use to take a tagola.Geometry and convert it to a wkt string. It will, also,
contain functions to parse a wkt string into a wkb.Geometry.
*/

func wkt(geo tegola.Geometry) string {

	switch g := geo.(type) {
	case tegola.Point:
		return fmt.Sprintf("%v %v", g.X(), g.Y())
	case tegola.Point3:
		return fmt.Sprintf("%v %v %v", g.X(), g.Y(), g.Z())
	case tegola.MultiPoint:
		var points []string
		for _, p := range g.Points() {
			points = append(points, wkt(p))
		}
		return "(" + strings.Join(points, ",") + ")"
	case tegola.LineString:
		var points []string
		for _, p := range g.Subpoints() {
			points = append(points, wkt(p))
		}
		return "(" + strings.Join(points, ",") + ")"

	case tegola.MultiLine:
		var lines []string
		for _, l := range g.Lines() {
			lines = append(lines, wkt(l))
		}
		return "(" + strings.Join(lines, ",") + ")"

	case tegola.Polygon:
		var lines []string
		for _, l := range g.Sublines() {
			lines = append(lines, wkt(l))
		}
		return "(" + strings.Join(lines, ",") + ")"
	case tegola.MultiPolygon:
		var polygons []string
		for _, p := range g.Polygons() {
			polygons = append(polygons, wkt(p))
		}
		return "(" + strings.Join(polygons, ",") + ")"
	}
	panic("Don't know the geometry type!")
}

//WKT returns a WKT representation of the Geometry if possible.
// the Error will be non-nil if geometry is unknown.
func WKT(geo tegola.Geometry) string {
	switch g := geo.(type) {
	default:
		return ""
	case tegola.Point:
		// POINT( 10 10)
		if g == nil {
			return "POINT EMPTY"
		}
		return "POINT (" + wkt(g) + ")"
	case tegola.Point3:
		// POINT M ( 10 10 10 )
		if g == nil {
			return "POINT M EMPTY"
		}
		return "POINT M (" + wkt(g) + ")"
	case tegola.MultiPoint:
		if g == nil {
			return "MULTIPOINT EMPTY"
		}
		return "MULTIPOINT " + wkt(g)
	case tegola.LineString:
		if g == nil {
			return "LINESTRING EMPTY"
		}
		return "LINESTRING " + wkt(g)
	case tegola.MultiLine:
		if g == nil {
			return "MULTILINE EMPTY"
		}
		return "MULTILINE " + wkt(g)
	case tegola.Polygon:
		if g == nil {
			return "POLYGON EMPTY"
		}
		return "POLYGON " + wkt(g)
	case tegola.MultiPolygon:
		if g == nil {
			return "MULTIPOLYGON EMPTY"
		}
		return "MULTIPOLYGON " + wkt(g)
	case tegola.Collection:
		if g == nil {
			return "GEOMETRYCOLLECTION EMPTY"

		}
		var geometries []string
		for sg := range g.Geometries() {
			s := WKT(sg)
			geometries = append(geometries, s)
		}
		return "GEOMETRYCOLLECTION (" + strings.Join(geometries, ",") + ")"
	}
}
