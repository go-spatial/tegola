// Package tegola describes the basic geometeries that can be used to convert to
// and from.
package tegola

import (
	"encoding/json"
	"fmt"
	"io"
)

// Geometry describes a geometry.
type Geometry interface{}

// Point is how a point should look like.
type Point interface {
	Geometry
	X() float64
	Y() float64
}

// Point3 is a point with three dimensions; at current is just converted and treated as a point.
type Point3 interface {
	Point
	Z() float64
}

// MultiPoint is a Geometry with multiple individual points.
type MultiPoint interface {
	Geometry
	Points() []Point
}

// MultiPoint is a Geometry with multiple individual points.
type MultiPoint3 interface {
	Geometry
	Points() []Point3
}

// LineString is a Geometry of a line.
type LineString interface {
	Geometry
	Subpoints() []Point
}

// MultiLine is a Geometry with multiple individual lines.
type MultiLine interface {
	Geometry
	Lines() []LineString
}

// Polygon is a multi-line Geometry  where all the lines connect to form an enclose space.
type Polygon interface {
	Geometry
	Sublines() []LineString
}

// MultiPolygon describes a Geometry multiple intersecting polygons. There should only one
// exterior polygon, and the rest of the polygons should be interior polygons. The interior
// polygons will exclude the area from the exterior polygon.
type MultiPolygon interface {
	Geometry
	Polygons() []Polygon
}

// Collection is a collections of different geometries.
type Collection interface {
	Geometry
	Geometries() []Geometry
}

func GeometryAsString(g Geometry) string {
	switch geo := g.(type) {
	case LineString:
		rstring := "["
		for _, p := range geo.Subpoints() {
			rstring = fmt.Sprintf("%v ( %v %v )", rstring, p.X(), p.Y())
		}
		rstring += "]"
		return rstring

	default:
		return fmt.Sprintf("%v", g)
	}
}

func GeometryAsMap(g Geometry) map[string]interface{} {
	js := make(map[string]interface{})
	var vals []map[string]interface{}
	switch geo := g.(type) {
	case Point:
		js["type"] = "point"
		js["value"] = []float64{geo.X(), geo.Y()}
	case Point3:
		js["type"] = "point3"
		js["value"] = []float64{geo.X(), geo.Y(), geo.Z()}
	case MultiPoint:
		js["type"] = "multipoint"
		for _, p := range geo.Points() {
			vals = append(vals, GeometryAsMap(p))
		}
		js["value"] = vals
	case LineString:
		js["type"] = "linestring"
		var fv []float64
		for _, p := range geo.Subpoints() {
			fv = append(fv, p.X(), p.Y())
		}
		js["value"] = fv
	case MultiLine:
		js["type"] = "multiline"
		for _, l := range geo.Lines() {
			vals = append(vals, GeometryAsMap(l))
		}
		js["value"] = vals
	case Polygon:
		js["type"] = "polygon"
		for _, l := range geo.Sublines() {
			vals = append(vals, GeometryAsMap(l))
		}
		js["value"] = vals
	case MultiPolygon:
		js["type"] = "multipolygon"
		for _, p := range geo.Polygons() {
			vals = append(vals, GeometryAsMap(p))
		}
		js["value"] = vals
	}
	return js
}

func GeometryAsJSON(g Geometry, w io.Writer) error {
	enc := json.NewEncoder(w)
	return enc.Encode(GeometryAsMap(g))
}
