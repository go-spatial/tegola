package geom

/*
 This file describes optional Interfaces to make geometries mutable.
*/

// PointSetter is a mutable Pointer.
type PointSetter interface {
	Pointer
	SetXY([2]float64) error
}

// PointZSetter is a mutable PointZer
type PointZSetter interface {
	PointZer
	SetXYZ([3]float64) error
}

// PointMSetter is a mutable PointMer
type PointMSetter interface {
	PointMer
	SetXYM([3]float64) error
}

// PointZMSetter is a mutable PointZMer
type PointZMSetter interface {
	PointZMer
	SetXYZM([4]float64) error
}

// PointSSetter is a mutable PointSer
type PointSSetter interface {
	PointSer
	SetXYS(srid uint32, xy Point) error
}

// PointZSSetter is a mutable PointZSer
type PointZSSetter interface {
	PointZSer
	SetXYZS(srid uint32, xyz PointZ) error
}

// PointMSSetter is a mutable PointMer
type PointMSSetter interface {
	PointMSer
	SetXYMS(srid uint32, xym PointM) error
}

// PointZMSSetter is a mutable PointZMer
type PointZMSSetter interface {
	PointZMSer
	SetXYZMS(srid uint32, xyzm PointZM) error
}

// MultiPointSetter is a mutable MultiPointer.
type MultiPointSetter interface {
	MultiPointer
	SetPoints([][2]float64) error
}

// MultiPointZSetter is a mutable MultiPointer.
type MultiPointZSetter interface {
	MultiPointZer
	SetPoints([][3]float64) error
}

// MultiPointMSetter is a mutable MultiPointer.
type MultiPointMSetter interface {
	MultiPointMer
	SetPoints([][3]float64) error
}

// MultiPointZMSetter is a mutable MultiPointer.
type MultiPointZMSetter interface {
	MultiPointZMer
	SetPoints([][4]float64) error
}

// MultiPointSSetter is a mutable MultiPointSer.
type MultiPointSSetter interface {
	MultiPointSer
	SetSRID(srid uint32, mp MultiPoint) error
}

// MultiPointZSSetter is a mutable MultiPointZSer.
type MultiPointZSSetter interface {
	MultiPointZSer
	SetSRID(srid uint32, mpz MultiPointZ) error
}

// MultiPointMSSetter is a mutable MultiPointMSer.
type MultiPointMSSetter interface {
	MultiPointMSer
	SetSRID(srid uint32, mpz MultiPointM) error
}

// MultiPointZMSSetter is a mutable MultiPointZMSer.
type MultiPointZMSSetter interface {
	MultiPointZMSer
	SetSRID(srid uint32, mpzm MultiPointZM) error
}

// LineStringSetter is a mutable LineStringer.
type LineStringSetter interface {
	LineStringer
	SetVertices([][2]float64) error
}

// LineStringMSetter is a mutable LineStringMer.
type LineStringMSetter interface {
	LineStringMer
	SetVertices([][3]float64) error
}

// LineStringZSetter is a mutable LineStringZer.
type LineStringZSetter interface {
	LineStringZer
	SetVertices([][3]float64) error
}

// LineStringZMSetter is a mutable LineStringZMer.
type LineStringZMSetter interface {
	LineStringZMer
	SetVertices([][4]float64) error
}

// LineStringSSetter is a mutable LineStringSer.
type LineStringSSetter interface {
	LineStringSer
	SetSRID(srid uint32, ls LineString) error
}

// LineStringMSSetter is a mutable LineStringMSer.
type LineStringMSSetter interface {
	LineStringMSer
	SetSRID(srid uint32, lsm LineStringM) error
}

// LineStringZSSetter is a mutable LineStringZSer.
type LineStringZSSetter interface {
	LineStringZSer
	SetSRID(srid uint32, lsz LineStringZ) error
}

// LineStringZMSSetter is a mutable LineStringZMSer.
type LineStringZMSSetter interface {
	LineStringZMSer
	SetSRID(srid uint32, lszm LineStringZM) error
}

// MultiLineStringSetter is a mutable MultiLineStringer.
type MultiLineStringSetter interface {
	MultiLineStringer
	SetLineStrings([][][2]float64) error
}

// MultiLineStringZSetter is a mutable MultiLineStringZer.
type MultiLineStringZSetter interface {
	MultiLineStringZer
	SetLineStringZs([][][3]float64) error
}

// MultiLineStringMSetter is a mutable MultiLineStringMer.
type MultiLineStringMSetter interface {
	MultiLineStringMer
	SetLineStringMs([][][3]float64) error
}

// MultiLineStringZMSetter is a mutable MultiLineZMStringer.
type MultiLineStringZMSetter interface {
	MultiLineStringZMer
	SetLineStringZMs([][][4]float64) error
}

// MultiLineStringSSetter is a mutable MultiLineSStringer.
type MultiLineStringSSetter interface {
	MultiLineStringSer
	SetSRID(srid uint32, mls MultiLineString) error
}

// MultiLineStringZSSetter is a mutable MultiLineZSStringer.
type MultiLineStringZSSetter interface {
	MultiLineStringZSer
	SetSRID(srid uint32, mlsz MultiLineStringZ) error
}

// MultiLineStringMSSetter is a mutable MultiLineMSStringer.
type MultiLineStringMSSetter interface {
	MultiLineStringMSer
	SetSRID(srid uint32, mlsm MultiLineStringM) error
}

// MultiLineStringZMSSetter is a mutable MultiLineZMSStringer.
type MultiLineStringZMSSetter interface {
	MultiLineStringZMSer
	SetSRID(srid uint32, mlszm MultiLineStringZM) error
}

// PolygonSetter is a mutable Polygoner.
type PolygonSetter interface {
	Polygoner
	SetLinearRings([][][2]float64) error
	AsSegments() ([][]Line, error)
}

type PolygonZSetter interface {
	PolygonZer
	SetLinearRings([][][3]float64) error
	AsSegments() ([][]LineZ, error)
}

type PolygonMSetter interface {
	PolygonMer
	SetLinearRings([][][3]float64) error
	AsSegments() ([][]LineM, error)
}

type PolygonZMSetter interface {
	PolygonZMer
	SetLinearRings([][][4]float64) error
	AsSegments() ([][]LineZM, error)
}

type PolygonSSetter interface {
	PolygonSer
	SetLinearRings(srid uint32, pol Polygon) error
	AsSegments() ([][]Line, uint32, error)
}

type PolygonZSSetter interface {
	PolygonZSer
	SetLinearRings(srid uint32, polz PolygonZ) error
	AsSegments() ([][]LineZ, uint32, error)
}

type PolygonMSSetter interface {
	PolygonMSer
	SetLinearRings(srid uint32, polm PolygonM) error
	AsSegments() ([][]LineM, uint32, error)
}

type PolygonZMSSetter interface {
	PolygonZMSer
	SetLinearRings(srid uint32, polzm PolygonZM) error
	AsSegments() ([][]LineZM, uint32, error)
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
