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
