package gpkg

import (
	"database/sql"
	"math"

	"github.com/gdey/errors"
	"github.com/go-spatial/geom"
)

var (
	nan     = math.NaN()
	emptyPt = [2]float64{nan, nan}
)

// Handle is the handle to the DB
type Handle struct {
	*sql.DB
}

type MaybeBool int8

const (
	No         = MaybeBool(0)
	Yes        = MaybeBool(1)
	Maybe      = MaybeBool(2)
	Prohibited = No
	Mandatory  = Yes
	Optional   = Maybe
)

func (mbe MaybeBool) True() bool {
	return mbe != No
}
func (mbe MaybeBool) False() bool {
	return !mbe.True()
}

// GeometryType describes the type of geometry to find for a table
type GeometryType uint8

const (
	// Geometry is the Normative type for a geometry
	Geometry = GeometryType(0)
	// Point is the Normative type for a point geometry
	Point = GeometryType(1)
	// Linestring is the Normative type for a linestring geometry
	Linestring = GeometryType(2)
	// Polygon is the Normative type for a polygon geometry
	Polygon = GeometryType(3)
	// MultiPoint is the Normative type for a multipoint geometry
	MultiPoint = GeometryType(4)
	// MultiLinestring is the Normative type for a multilinestring geometry
	MultiLinestring = GeometryType(5)
	// MultiPolygon is the Normative type for a multipolygon geometry
	MultiPolygon = GeometryType(6)
	// GeometryCollection is the Normative type for a collection of geometries
	GeometryCollection = GeometryType(7)
)

func (gt GeometryType) String() string {
	switch gt {
	case Geometry:
		return "GEOMETRY"
	case Point:
		return "POINT"
	case Linestring:
		return "LINESTRING"
	case Polygon:
		return "POLYGON"
	case MultiPoint:
		return "MULTIPOINT"
	case MultiLinestring:
		return "MULTILINESTRING"
	case MultiPolygon:
		return "MULTIPOLYGON"
	case GeometryCollection:
		return "GEOMETRYCOLLECTION"
	default:
		return "UNKNOWN"
	}
}

func TypeForGeometry(g geom.Geometry) GeometryType {
	switch g.(type) {
	case geom.Collectioner:
		return GeometryCollection
	case geom.MultiPolygoner:
		return MultiPolygon
	case geom.MultiLineStringer:
		return MultiLinestring
	case geom.MultiPointer:
		return MultiPoint
	case geom.Polygoner:
		return Polygon
	case geom.LineStringer:
		return Linestring
	case geom.Pointer:
		return Point
	default:
		return Geometry
	}
}

func (gt GeometryType) Empty() (geom.Geometry, error) {
	switch gt {
	case Point:
		// return geom.EmptyPoint,nil
		return geom.Point(emptyPt), nil
	case Linestring:
		return geom.LineString{}, nil
	case Polygon:
		return geom.Polygon{}, nil
	case MultiLinestring:
		return geom.MultiLineString{}, nil
	case MultiPolygon:
		return geom.MultiPolygon{}, nil
	case GeometryCollection:
		return geom.Collection{}, nil
	default:
		return nil, errors.String("need concreate geometry")
	}
}

// TableDescription describes a content table
type TableDescription struct {
	Name          string
	ShortName     string
	Description   string
	GeometryField string
	GeometryType  GeometryType
	SRS           int32
	Z             MaybeBool
	M             MaybeBool
}
