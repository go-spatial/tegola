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

// PointZer is a 3D point.
type PointZer interface {
	Geometry
	XYZ() [3]float64
}

// PointMer is a 2D+1D point.
type PointMer interface {
	Geometry
	XYM() [3]float64
}

// PointZMer is a 3D+1D point.
type PointZMer interface {
	Geometry
	XYZM() [4]float64
}

// PointSer is a 2D point + SRID
type PointSer interface {
	Geometry
	XYS() struct {
		Srid uint32
		Xy   Point
	}
}

// PointZSer is a 3D point + SRID
type PointZSer interface {
	Geometry
	XYZS() struct {
		Srid uint32
		Xyz  PointZ
	}
}

// PointMSer is a 2D+1D point + SRID
type PointMSer interface {
	Geometry
	XYMS() struct {
		Srid uint32
		Xym  PointM
	}
}

// PointZMSer is a 3D+1D point + SRID
type PointZMSer interface {
	Geometry
	XYZMS() struct {
		Srid uint32
		Xyzm PointZM
	}
}

// MultiPointer is a geometry with multiple points.
type MultiPointer interface {
	Geometry
	Points() [][2]float64
}

// MultiPointZer is a geometry with multiple 3D points.
type MultiPointZer interface {
	Geometry
	Points() [][3]float64
}

// MultiPointMer is a geometry with multiple 2+1D points.
type MultiPointMer interface {
	Geometry
	Points() [][3]float64
}

// MultiPointZMer is a geometry with multiple 3+1D points.
type MultiPointZMer interface {
	Geometry
	Points() [][4]float64
}

// MultiPointSer is a MultiPoint + SRID.
type MultiPointSer interface {
	Geometry
	Points() struct {
		Srid uint32
		Mp   MultiPoint
	}
}

// MultiPointZSer is a MultiPointZ + SRID.
type MultiPointZSer interface {
	Geometry
	Points() struct {
		Srid uint32
		Mpz  MultiPointZ
	}
}

// MultiPointMSer is a MultiPointM + SRID.
type MultiPointMSer interface {
	Geometry
	Points() struct {
		Srid uint32
		Mpm  MultiPointM
	}
}

// MultiPointZMSer is a MultiPointZM + SRID.
type MultiPointZMSer interface {
	Geometry
	Points() struct {
		Srid uint32
		Mpzm MultiPointZM
	}
}

// LineStringer is a line of two or more points.
type LineStringer interface {
	Geometry
	Vertices() [][2]float64
}

// LineStringMer is a line of two or more M points.
type LineStringMer interface {
	Geometry
	Vertices() [][3]float64
}

// LineStringZer is a line of two or more Z points.
type LineStringZer interface {
	Geometry
	Vertices() [][3]float64
}

// LineStringZMer is a line of two or more ZM points.
type LineStringZMer interface {
	Geometry
	Vertices() [][4]float64
}

// LineStringSer is a line of two or more points + SRID.
type LineStringSer interface {
	Geometry
	Vertices() struct {
		Srid uint32
		Ls   LineString
	}
}

// LineStringMSer is a line of two or more M points + SRID.
type LineStringMSer interface {
	Geometry
	Vertices() struct {
		Srid uint32
		Lsm  LineStringM
	}
}

// LineStringZSer is a line of two or more Z points + SRID.
type LineStringZSer interface {
	Geometry
	Vertices() struct {
		Srid uint32
		Lsz  LineStringZ
	}
}

// LineStringZMSer is a line of two or more ZM points + SRID.
type LineStringZMSer interface {
	Geometry
	Vertices() struct {
		Srid uint32
		Lszm LineStringZM
	}
}

// MultiLineStringer is a geometry with multiple LineStrings.
type MultiLineStringer interface {
	Geometry
	LineStrings() [][][2]float64
}

// MultiLineZStringer is a geometry with multiple LineZStrings.
type MultiLineStringZer interface {
	Geometry
	LineStringZs() [][][3]float64
}

// MultiLineMStringer is a geometry with multiple LineMStrings.
type MultiLineStringMer interface {
	Geometry
	LineStringMs() [][][3]float64
}

// MultiLineZMStringer is a geometry with multiple LineZMStrings.
type MultiLineStringZMer interface {
	Geometry
	LineStringZMs() [][][4]float64
}

// MultiLineSStringer is a geometry with multiple LineSStrings.
type MultiLineStringSer interface {
	Geometry
	MultiLineStrings() struct {
		Srid uint32
		Mls  MultiLineString
	}
}

// MultiLineZSStringer is a geometry with multiple LineZSStrings.
type MultiLineStringZSer interface {
	Geometry
	MultiLineStringZs() struct {
		Srid uint32
		Mlsz MultiLineStringZ
	}
}

// MultiLineMSStringer is a geometry with multiple LineMSStrings.
type MultiLineStringMSer interface {
	Geometry
	MultiLineStringMs() struct {
		Srid uint32
		Mlsm MultiLineStringM
	}
}

// MultiLineZMSStringer is a geometry with multiple LineZMSStrings.
type MultiLineStringZMSer interface {
	Geometry
	MultiLineStringZMs() struct {
		Srid  uint32
		Mlszm MultiLineStringZM
	}
}

// Polygoner is a geometry consisting of multiple Linear Rings.
// There must be only one exterior LineString with a clockwise winding order.
// There may be one or more interior LineStrings with a counterclockwise winding orders.
// It is assumed that the last point is connected to the first point, and the first point is NOT duplicated at the end.
type Polygoner interface {
	Geometry
	LinearRings() [][][2]float64
}

type PolygonZer interface {
	Geometry
	LinearRings() [][][3]float64
}

type PolygonMer interface {
	Geometry
	LinearRings() [][][3]float64
}

type PolygonZMer interface {
	Geometry
	LinearRings() [][][4]float64
}

type PolygonSer interface {
	Geometry
	LinearRings() struct {
		Srid uint32
		Pol  Polygon
	}
}

type PolygonZSer interface {
	Geometry
	LinearRings() struct {
		Srid uint32
		Polz PolygonZ
	}
}

type PolygonMSer interface {
	Geometry
	LinearRings() struct {
		Srid uint32
		Polm PolygonM
	}
}

type PolygonZMSer interface {
	Geometry
	LinearRings() struct {
		Srid  uint32
		Polzm PolygonZM
	}
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

// IsNil is a helper function to check it the given interface is nil, or the
// value store in it is nil
func IsNil(a interface{}) bool {
	defer func() { recover() }()
	return a == nil || reflect.ValueOf(a).IsNil()
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
