package geom

/*
 This file describes optional Interfaces to make geometries mutable.
*/

// PointSetter is a mutable Pointer.
type PointSetter interface {
	Pointer
	SetXY([2]float64) error
}

// MultiPointSetter is a mutable MultiPointer.
type MultiPointSetter interface {
	MultiPointer
	SetPoints([][2]float64) error
}

// LineStringSetter is a mutable LineStringer.
type LineStringSetter interface {
	LineStringer
	SetVertices([][2]float64) error
}

// MultiLineStringSetter is a mutable MultiLineStringer.
type MultiLineStringSetter interface {
	MultiLineStringer
	SetLineStrings([][][2]float64) error
}

// PolygonSetter is a mutable Polygoner.
type PolygonSetter interface {
	Polygoner
	SetLinearRings([][][2]float64) error
}

// MultiPolygonSetter is a mutable MultiPolygoner.
type MultiPolygonSetter interface {
	MultiPolygoner
	SetPolygons([][][][2]float64) error
}

// CollectionSetter is a mutable Collectioner.
type CollectionSetter interface {
	Collectioner
	SetGeometries([]Geometry) error
}
