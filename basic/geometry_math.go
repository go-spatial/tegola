package basic

import (
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths/webmercator"
)

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints(geometry tegola.Geometry, f func(coords ...float64) ([]float64, error)) (tegola.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %+v", geometry)
	case tegola.Point:
		c, err := f(geo.X(), geo.Y())
		if err != nil {
			return nil, err
		}
		if len(c) < 2 {
			return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
		}
		return &Point{c[0], c[1]}, nil
	case tegola.Point3:
		c, err := f(geo.X(), geo.Y(), geo.Z())
		if err != nil {
			return nil, err
		}
		if len(c) < 3 {
			return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 3", len(c))
		}
		return &Point3{c[0], c[1], c[2]}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for _, pt := range geo.Points() {
			c, err := f(pt.X(), pt.Y())
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			pts = append(pts, Point{c[0], c[1]})
		}
		return pts, nil
	case tegola.LineString:
		var line Line
		for _, ptGeo := range geo.Subpoints() {
			c, err := f(ptGeo.X(), ptGeo.Y())
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			line = append(line, Point{c[0], c[1]})
		}
		return &line, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geoLine, err := ApplyToPoints(lineGeo, f)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of multiline: %v", i, err)
			}
			ln, ok := geoLine.(Line)
			if !ok {
				panic("We did not get the conversion we were expecting")
			}
			line = append(line, ln)
		}
		return line, nil
	case tegola.Polygon:
		var poly Polygon
		for i, line := range geo.Sublines() {
			geoLine, err := ApplyToPoints(line, f)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			ln, ok := geoLine.(*Line)
			if !ok {
				panic("We did not get the conversion we were expecting")
			}

			poly = append(poly, *ln)
		}
		return &poly, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geoPoly, err := ApplyToPoints(poly, f)
			if err != nil {
				return nil, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			p, ok := geoPoly.(*Polygon)
			if !ok {
				panic("We did not get the conversion we were expecting")
			}
			mpoly = append(mpoly, *p)
		}
		return &mpoly, nil
	}
}

// CloneGeomtry returns a deep clone of the Geometry.
func CloneGeometry(geometry tegola.Geometry) (tegola.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %+v", geometry)
	case tegola.Point:
		return &Point{geo.X(), geo.Y()}, nil
	case tegola.Point3:
		return &Point3{geo.X(), geo.Y(), geo.Z()}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for _, pt := range geo.Points() {
			pts = append(pts, Point{pt.X(), pt.Y()})
		}
		return pts, nil
	case tegola.LineString:
		var line Line
		for _, ptGeo := range geo.Subpoints() {
			line = append(line, Point{ptGeo.X(), ptGeo.Y()})
		}
		return &line, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geoLine, err := CloneGeometry(lineGeo)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of multiline: %v", i, err)
			}
			ln, _ := geoLine.(Line)
			line = append(line, ln)
		}
		return line, nil
	case tegola.Polygon:
		var poly Polygon
		for i, line := range geo.Sublines() {
			geoLine, err := CloneGeometry(line)
			if err != nil {
				return nil, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			ln, _ := geoLine.(Line)
			poly = append(poly, ln)
		}
		return &poly, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geoPoly, err := CloneGeometry(poly)
			if err != nil {
				return nil, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			p, _ := geoPoly.(Polygon)
			mpoly = append(mpoly, p)
		}
		return &mpoly, nil
	}
}

// ToWebMercator takes a SRID and a geometry encode using that srid, and returns a geometry encoded as a WebMercator.
func ToWebMercator(SRID int, geometry tegola.Geometry) (tegola.Geometry, error) {
	switch SRID {
	default:
		return nil, fmt.Errorf("Don't know how to convert from %v to %v.", tegola.WebMercator, SRID)
	case tegola.WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.
		return CloneGeometry(geometry)
	case tegola.WGS84:
		return ApplyToPoints(geometry, webmercator.PToXY)
	}
}

// FromWebMercator takes a geometry encoded with WebMercator, and returns a Geometry encodes to the given srid.
func FromWebMercator(SRID int, geometry tegola.Geometry) (tegola.Geometry, error) {
	switch SRID {
	default:
		return nil, fmt.Errorf("Don't know how to convert from %v to %v.", SRID, tegola.WebMercator)
	case tegola.WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.
		return CloneGeometry(geometry)
	case tegola.WGS84:
		return ApplyToPoints(geometry, webmercator.PToLonLat)
	}
}
