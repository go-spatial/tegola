package wkt

import (
	"fmt"
	"strings"

	"github.com/terranodo/tegola/geom"
)

/*
This purpose of this file is to house the wkt functions. These functions are
use to take a tagola.Geometry and convert it to a wkt string. It will, also,
contain functions to parse a wkt string into a wkb.Geometry.
*/

type UnknownGeometryError struct {
	Geom geom.Geometry
}

func (e UnknownGeometryError) Error() string {
	return fmt.Sprintf("Unknown Geometry! %v", e.Geom)
}

func _encode(geo geom.Geometry) string {
	switch g := geo.(type) {
	case geom.Pointer:
		xy := g.XY()
		return fmt.Sprintf("%v %v", xy[0], xy[1])
	case geom.MultiPointer:
		var points []string
		for _, p := range g.Points() {
			points = append(points, _encode(p))
		}
		return "(" + strings.Join(points, ",") + ")"
	case geom.LineStringer:
		var points []string
		for _, p := range g.Verticies() {
			points = append(points, _encode(p))
		}
		return "(" + strings.Join(points, ",") + ")"

	case geom.MultiLineStringer:
		var lines []string
		for _, l := range g.LineStrings() {
			lines = append(lines, _encode(l))
		}
		return "(" + strings.Join(lines, ",") + ")"

	case geom.Polygoner:
		var rings []string
		for _, l := range g.LinearRings() {
			rings = append(rings, _encode(l))
		}
		return "(" + strings.Join(rings, ",") + ")"
	case geom.MultiPolygoner:
		var polygons []string
		for _, p := range g.Polygons() {
			polygons = append(polygons, _encode(p))
		}
		return "(" + strings.Join(polygons, ",") + ")"
	}
	panic("Don't know the geometry type!")
}

//WKT returns a WKT representation of the Geometry if possible.
// the Error will be non-nil if geometry is unknown.
func Encode(geo geom.Geometry) (string, error) {
	switch g := geo.(type) {
	default:
		return "", UnknownGeometryError{geo}
	case geom.Pointer:
		// POINT( 10 10)
		if g == nil {
			return "POINT EMPTY", nil
		}
		return "POINT ( %v %v )" + _encode(g) + ")", nil
	case geom.MultiPointer:
		if g == nil {
			return "MULTIPOINT EMPTY", nil
		}
		return "MULTIPOINT " + _encode(g), nil
	case geom.LineStringer:
		if g == nil {
			return "LINESTRING EMPTY", nil
		}
		return "LINESTRING " + _encode(g), nil
	case geom.MultiLineStringer:
		if g == nil {
			return "MULTILINE EMPTY", nil
		}
		return "MULTILINE " + _encode(g), nil
	case geom.Polygoner:
		if g == nil {
			return "POLYGON EMPTY", nil
		}
		return "POLYGON " + _encode(g), nil
	case geom.MultiPolygoner:
		if g == nil {
			return "MULTIPOLYGON EMPTY", nil
		}
		return "MULTIPOLYGON " + _encode(g), nil
	case geom.Collectioner:
		if g == nil {
			return "GEOMETRYCOLLECTION EMPTY", nil
		}
		var geometries []string
		for sg := range g.Geometries() {
			s, err := Encode(sg)
			if err != nil {
				return "", err
			}
			geometries = append(geometries, s)
		}
		return "GEOMETRYCOLLECTION (" + strings.Join(geometries, ",") + ")", nil
	}
}

func Decode(text string) (geo geom.Geometry, err error) {
	return nil, nil
}
