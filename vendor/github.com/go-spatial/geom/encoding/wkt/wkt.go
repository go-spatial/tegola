package wkt

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
)

func isNil(a interface{}) bool {
	defer func() { recover() }()
	return a == nil || reflect.ValueOf(a).IsNil()
}

func isMultiLineStringerEmpty(ml geom.MultiLineStringer) bool {
	if isNil(ml) || len(ml.LineStrings()) == 0 {
		return true
	}
	lns := ml.LineStrings()
	// It's not nil, and there are several lines.
	// We need to go through all the lines and make sure that at least one of them has a non-zero length.
	for i := range lns {
		if len(lns[i]) != 0 {
			return false
		}
	}
	return true
}

func isPolygonerEmpty(p geom.Polygoner) bool {
	if isNil(p) || len(p.LinearRings()) == 0 {
		return true
	}
	lns := p.LinearRings()
	// It's not nil, and there are several lines.
	// We need to go through all the lines and make sure that at least one of them has a non-zero length.
	for i := range lns {
		if len(lns[i]) != 0 {
			return false
		}
	}
	return true
}

func isMultiPolygonerEmpty(mp geom.MultiPolygoner) bool {
	if isNil(mp) || len(mp.Polygons()) == 0 {
		return true
	}
	plys := mp.Polygons()
	for i := range plys {
		for j := range plys[i] {
			if len(plys[i][j]) != 0 {
				return false
			}
		}
	}
	return true
}

func isCollectionerEmpty(col geom.Collectioner) bool {
	if isNil(col) || len(col.Geometries()) == 0 {
		return true
	}
	geos := col.Geometries()
	for i := range geos {
		switch g := geos[i].(type) {
		case geom.Pointer:
			if !isNil(g) {
				return false
			}
		case geom.MultiPointer:
			if !(isNil(g) || len(g.Points()) == 0) {
				return false
			}
		case geom.LineStringer:
			if !(isNil(g) || len(g.Verticies()) == 0) {
				return false
			}
		case geom.MultiLineStringer:
			if !isMultiLineStringerEmpty(g) {
				return false
			}
		case geom.Polygoner:
			if !isPolygonerEmpty(g) {
				return false
			}
		case geom.MultiPolygoner:
			if !isMultiPolygonerEmpty(g) {
				return false
			}
		case geom.Collectioner:
			if !isCollectionerEmpty(g) {
				return false
			}
		}
	}
	return true
}

func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 3, 64)
	if s[len(s)-3:] == "000" {
		// remove the .
		return s[:len(s)-4]
	}
	return s
}

func formatPoint(pt [2]float64) string {
	return formatFloat(pt[0]) + " " + formatFloat(pt[1])
}

/*
This purpose of this file is to house the wkt functions. These functions are
use to take a tagola.Geometry and convert it to a wkt string. It will, also,
contain functions to parse a wkt string into a wkb.Geometry.
*/

func _encode(geo geom.Geometry) string {

	switch g := geo.(type) {

	case geom.Pointer:
		return formatPoint(g.XY())

	case [2]float64:
		return formatPoint(g)

	case geom.MultiPointer:
		var points []string
		for _, p := range g.Points() {
			points = append(points, _encode(geom.Point(p)))
		}
		return "(" + strings.Join(points, ",") + ")"

	case geom.LineStringer:
		var points []string
		for _, p := range g.Verticies() {
			points = append(points, _encode(geom.Point(p)))
		}
		return "(" + strings.Join(points, ",") + ")"

	case geom.MultiLineStringer:
		var lines []string
		for _, l := range g.LineStrings() {
			if len(l) == 0 {
				continue
			}
			lines = append(lines, _encode(geom.LineString(l)))
		}
		return "(" + strings.Join(lines, ",") + ")"

	case geom.Polygoner:
		var rings []string
		for _, l := range g.LinearRings() {
			if len(l) == 0 {
				continue
			}
			if !cmp.PointEqual(l[0], l[len(l)-1]) {
				// Dup the first point to close the polygon.
				l = append(l, l[0])
			}
			rings = append(rings, _encode(geom.LineString(l)))
		}
		return "(" + strings.Join(rings, ",") + ")"

	case geom.MultiPolygoner:
		var polygons []string
		for _, p := range g.Polygons() {
			if len(p) == 0 {
				continue
			}
			polygons = append(polygons, _encode(geom.Polygon(p)))
		}
		return "(" + strings.Join(polygons, ",") + ")"

	}
	panic(fmt.Sprintf("Don't know the geometry type! %+v", geo))
}

//WKT returns a WKT representation of the Geometry if possible.
// the Error will be non-nil if geometry is unknown.
func Encode(geo geom.Geometry) (string, error) {
	switch g := geo.(type) {
	default:

		return "", geom.ErrUnknownGeometry{geo}

	case geom.Pointer:

		// POINT( 10 10)
		if isNil(g) {
			return "POINT EMPTY", nil
		}
		return "POINT (" + _encode(geo) + ")", nil

	case [2]float64:

		return "POINT (" + _encode(geo) + ")", nil

	case geom.MultiPointer:

		if isNil(g) || len(g.Points()) == 0 {
			return "MULTIPOINT EMPTY", nil
		}
		return "MULTIPOINT " + _encode(geo), nil

	case geom.LineStringer:

		if isNil(g) || len(g.Verticies()) == 0 {
			return "LINESTRING EMPTY", nil
		}
		return "LINESTRING " + _encode(geo), nil

	case geom.MultiLineStringer:

		if isMultiLineStringerEmpty(g) {
			return "MULTILINESTRING EMPTY", nil
		}
		return "MULTILINESTRING " + _encode(geo), nil

	case geom.Polygoner:

		if isPolygonerEmpty(g) {
			return "POLYGON EMPTY", nil
		}
		return "POLYGON " + _encode(geo), nil

	case geom.MultiPolygoner:

		if isMultiPolygonerEmpty(g) {
			return "MULTIPOLYGON EMPTY", nil
		}
		return "MULTIPOLYGON " + _encode(geo), nil

	case geom.Collectioner:

		if isCollectionerEmpty(g) {
			return "GEOMETRYCOLLECTION EMPTY", nil
		}
		var geometries []string
		for _, sg := range g.Geometries() {
			s, err := Encode(sg)
			if err != nil {
				return "", err
			}
			geometries = append(geometries, s)
		}
		return "GEOMETRYCOLLECTION (" + strings.Join(geometries, ",") + ")", nil

	case geom.Line:

		return Encode(geom.LineString(g[:]))

	case [2][2]float64:

		return Encode(geom.LineString(g[:]))

	case [][2]float64:

		return Encode(geom.LineString(g))

	case []geom.Line:

		ml := make(geom.MultiLineString, len(g))
		for i := range g {
			ml[i] = g[i][:]
		}
		return Encode(ml)

	case []geom.Point:
		mp := make(geom.MultiPoint, len(g))
		for i := range g {
			mp[i] = [2]float64(g[i])
		}
		return Encode(mp)

	case geom.Triangle:
		// treat a triangle as polygon
		return Encode(geom.Polygon{g[:]})

	case []geom.Triangle:
		mp := make(geom.MultiPolygon, len(g))
		for i := range g {
			mp[i] = geom.Polygon{g[i][:]}
		}
		return Encode(mp)
	case geom.Extent:
		// treat an extent as a ploygon
		return Encode(g.AsPolygon())
	case *geom.Extent:
		// treat an extent as a ploygon
		if g != nil {
			return Encode(g.AsPolygon())
		}
		return Encode(geom.Polygon{})
	}
}
func MustEncode(geo geom.Geometry) (str string) {
	var err error
	if str, err = Encode(geo); err != nil {
		panic(fmt.Sprintf("unable to encode %T as wkt", geo))
	}
	return str
}

func Decode(text string) (geo geom.Geometry, err error) {
	return nil, nil
}
