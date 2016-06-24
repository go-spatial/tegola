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
		for p := range g.Points() {
			points = append(points, wkt(p))
		}
		return "(" + strings.Join(points, ",") + ")"
	case tegola.LineString:
		var points []string
		for p := range g.Subpoints() {
			points = append(points, wkt(p))
		}
		return "(" + strings.Join(points, ",") + ")"

	case tegola.MultiLine:
		var lines []string
		for l := range g.Lines() {
			lines = append(lines, wkt(l))
		}
		return "(" + strings.Join(lines, ",") + ")"

	case tegola.Polygon:
		var lines []string
		for l := range g.Sublines() {
			lines = append(lines, wkt(l))
		}
		return "(" + strings.Join(lines, ",") + ")"
	case tegola.MultiPolygon:
		var polygons []string
		for p := range g.Polygons() {
			polygons = append(polygons, wkt(p))
		}
		return "(" + strings.Join(polygons, ",") + ")"
	}
	panic("Don't know the geometry type!")
}

func WKT(geo tegola.Geometry) (string, error) {
	switch g := geo.(type) {
	case tegola.Point:
		// POINT( 10 10)
		if g == nil {
			return "POINT EMPTY", nil
		}
		return "POINT (" + wkt(g) + ")", nil
	case tegola.Point3:
		// POINT M ( 10 10 10 )
		if g == nil {
			return "POINT M EMPTY", nil
		}
		return "POINT M (" + wkt(g) + ")", nil
	case tegola.MultiPoint:
		if g == nil {
			return "MULTIPOINT EMPTY", nil
		}
		return "MULTIPOINT " + wkt(g), nil
	case tegola.LineString:
		if g == nil {
			return "LINESTRING EMPTY", nil
		}
		return "LINESTRING " + wkt(g), nil
	case tegola.MultiLine:
		if g == nil {
			return "MULTILINE EMPTY", nil
		}
		return "MULTILINE " + wkt(g), nil
	case tegola.Polygon:
		if g == nil {
			return "POLYGON EMPTY", nil
		}
		return "POLYGON " + wkt(g), nil
	case tegola.MultiPolygon:
		if g == nil {
			return "MULTIPOLYGON EMPTY", nil
		}
		return "MULTIPOLYGON " + wkt(g), nil
	case tegola.Collection:
		if g == nil {
			return "GEOMETRYCOLLECTION EMPTY", nil

		}
		var geometries []string
		for sg := range g.Geometries() {
			s, err := WKT(sg)
			if err != nil {
				return "", err
			}
			geometries = append(geometries, s)
		}
		return "GEOMETRYCOLLECTION (" + strings.Join(geometries, ",") + ")", nil
	}
	return "UNKNOWN Type", fmt.Errorf("Unknow Geometry Type %v", geo)
}
