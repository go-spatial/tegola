// Package geom describes geometry interfaces.
package geom

import (
	"math"
	"reflect"
)

// Geometry is an object with a spatial reference.
// if a method accepts a Geometry type it's only expected to support the geom types in this package
type Geometry interface{}

// Pointer is a point with two dimensions.
type Pointer interface {
	Geometry
	XY() [2]float64
}

// MultiPointer is a geometry with multiple points.
type MultiPointer interface {
	Geometry
	Points() [][2]float64
}

// LineStringer is a line of two or more points.
type LineStringer interface {
	Geometry
	Vertices() [][2]float64
}

// MultiLineStringer is a geometry with multiple LineStrings.
type MultiLineStringer interface {
	Geometry
	LineStrings() [][][2]float64
}

// Polygoner is a geometry consisting of multiple Linear Rings.
// There must be only one exterior LineString with a clockwise winding order.
// There may be one or more interior LineStrings with a counterclockwise winding orders.
// It is assumed that the last point is connected to the first point, and the first point is NOT duplicated at the end.
type Polygoner interface {
	Geometry
	LinearRings() [][][2]float64
}

// MultiPolygoner is a geometry of multiple polygons.
type MultiPolygoner interface {
	Geometry
	Polygons() [][][][2]float64
}

// Collectioner is a collections of different geometries.
type Collectioner interface {
	Geometry
	Geometries() []Geometry
}

// getCoordinates is a helper function for GetCoordinates to avoid too many
// array copies and still provide a convenient interface to the user.
func getCoordinates(g Geometry, pts *[]Point) error {
	switch gg := g.(type) {

	default:

		return ErrUnknownGeometry{g}

	case Pointer:

		*pts = append(*pts, Point(gg.XY()))
		return nil

	case MultiPointer:

		mpts := gg.Points()
		for i := range mpts {
			*pts = append(*pts, Point(mpts[i]))
		}
		return nil

	case LineStringer:

		mpts := gg.Vertices()
		for i := range mpts {
			*pts = append(*pts, Point(mpts[i]))
		}
		return nil

	case MultiLineStringer:

		for _, ls := range gg.LineStrings() {
			if err := getCoordinates(LineString(ls), pts); err != nil {
				return err
			}
		}
		return nil

	case Polygoner:

		for _, ls := range gg.LinearRings() {
			if err := getCoordinates(LineString(ls), pts); err != nil {
				return err
			}
		}
		return nil

	case MultiPolygoner:

		for _, p := range gg.Polygons() {
			if err := getCoordinates(Polygon(p), pts); err != nil {
				return err
			}
		}
		return nil

	case Collectioner:

		for _, child := range gg.Geometries() {
			if err := getCoordinates(child, pts); err != nil {
				return err
			}
		}
		return nil

	}
}

/*
GetCoordinates return a list of points that make up a geometry. This makes no
attempt to remove duplicate points.
*/
func GetCoordinates(g Geometry) (pts []Point, err error) {
	// recursively retrieve points.
	err = getCoordinates(g, &pts)
	return pts, err
}

// getExtent is a helper function to efficiently build an Extent without
// collecting all coordinates first.
func getExtent(g Geometry, e *Extent) error {
	switch gg := g.(type) {

	default:

		return ErrUnknownGeometry{g}

	case Pointer:
		e.AddPoints(gg.XY())
		return nil

	case MultiPointer:
		e.AddPoints(gg.Points()...)
		return nil

	case LineStringer:
		e.AddPoints(gg.Vertices()...)
		return nil

	case MultiLineStringer:

		for _, ls := range gg.LineStrings() {
			if err := getExtent(LineString(ls), e); err != nil {
				return err
			}
		}
		return nil

	case Polygoner:

		for _, ls := range gg.LinearRings() {
			if err := getExtent(LineString(ls), e); err != nil {
				return err
			}
		}
		return nil

	case MultiPolygoner:

		for _, p := range gg.Polygons() {
			if err := getExtent(Polygon(p), e); err != nil {
				return err
			}
		}
		return nil

	case Collectioner:

		for _, child := range gg.Geometries() {
			if err := getExtent(child, e); err != nil {
				return err
			}
		}
		return nil

	}
}

// extractLines is a helper function for ExtractLines to avoid too many
// array copies and still provide a convenient interface to the user.
func extractLines(g Geometry, lines *[]Line) error {
	switch gg := g.(type) {

	default:

		return ErrUnknownGeometry{g}

	case Pointer:

		return nil

	case MultiPointer:

		return nil

	case LineStringer:

		v := gg.Vertices()
		for i := 0; i < len(v)-1; i++ {
			*lines = append(*lines, Line{v[i], v[i+1]})
		}
		return nil

	case MultiLineStringer:

		for _, ls := range gg.LineStrings() {
			if err := extractLines(LineString(ls), lines); err != nil {
				return err
			}
		}
		return nil

	case Polygoner:

		for _, v := range gg.LinearRings() {
			lr := LineString(v)
			if err := extractLines(lr, lines); err != nil {
				return err
			}
			v := lr.Vertices()
			if len(v) > 2 && lr.IsRing() == false {
				// create a connection from last -> first if it doesn't exist
				*lines = append(*lines, Line{v[len(v)-1], v[0]})
			}
		}
		return nil

	case MultiPolygoner:

		for _, p := range gg.Polygons() {
			if err := extractLines(Polygon(p), lines); err != nil {
				return err
			}
		}
		return nil

	case Collectioner:

		for _, child := range gg.Geometries() {
			if err := extractLines(child, lines); err != nil {
				return err
			}
		}
		return nil

	}
}

// ExtractLines extracts all linear components from a geometry (line segements).
// If the geometry contains no line segements (e.g. empty geometry or
// point), then an empty array will be returned.
//
// Duplicate lines will not be removed.
func ExtractLines(g Geometry) (lines []Line, err error) {
	err = extractLines(g, &lines)
	return lines, err
}

// helper function to check it the given interface is nil, or the
// value store in it is nil
func isNil(a interface{}) bool {
	defer func() { recover() }()
	return a == nil || reflect.ValueOf(a).IsNil()
}

// IsEmpty returns if the geometry represents an empty geometry
func IsEmpty(geo Geometry) bool {
	if isNil(geo) {
		return true
	}
	switch g := geo.(type) {
	case Point:
		return g[0] == nan && g[1] == nan
	case Pointer:
		xy := g.XY()
		return xy[0] == nan && xy[1] == nan
	case LineString:
		return len(g) == 0
	case LineStringer:
		return len(g.Vertices()) == 0
	case Polygon:
		return len(g) == 0
	case Polygoner:
		return len(g.LinearRings()) == 0
	case MultiPoint:
		return len(g) == 0
	case MultiPointer:
		return len(g.Points()) == 0
	case MultiLineString:
		return len(g) == 0
	case MultiLineStringer:
		return len(g.LineStrings()) == 0
	case MultiPolygon:
		return len(g) == 0
	case MultiPolygoner:
		return len(g.Polygons()) == 0
	case Collection:
		return len(g) == 0
	case Collectioner:
		return len(g.Geometries()) == 0
	default:
		return true
	}
}

// RoundToPrec will round the given value to the precision value.
// The precision value should be a power of 10.
func RoundToPrec(v float64, prec int) float64 {
	if v == -0.0 {
		return 0.0
	}
	if prec == 0 {
		return math.Round(v)
	}
	RoundingFactor := math.Pow10(prec)
	return math.Round(v*RoundingFactor) / RoundingFactor
}
