// Package geom describes geometry interfaces.
package geom

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
	Verticies() [][2]float64
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

		mpts := gg.Verticies()
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

		v := gg.Verticies()
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
			v := lr.Verticies()
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

/*
ExtractLines extracts all linear components from a geometry (line segements).
If the geometry contains no line segements (e.g. empty geometry or
point), then an empty array will be returned.

Duplicate lines will not be removed.
*/
func ExtractLines(g Geometry) (lines []Line, err error) {
	err = extractLines(g, &lines)
	return lines, err
}
