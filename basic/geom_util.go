package basic

import (
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths/webmercator"
)

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints1(geometry geom.Geometry, f func(coords ...float64) ([]float64, error)) (geom.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %+v", geometry)

	case geom.Point:
		c, err := f(geo.X(), geo.Y())
		if err != nil {
			return nil, err
		}
		if len(c) < 2 {
			return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
		}
		return geom.Point{c[0], c[1]}, nil

	case geom.MultiPoint:
		pts := make(geom.MultiPoint, len(geo))

		for i, pt := range geo {
			c, err := f(pt[:]...)
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			pts[i][0], pts[i][1] = c[0], c[1]
		}
		return pts, nil

	case geom.LineString:
		line := make(geom.LineString, len(geo))
		for i, pt := range geo {
			c, err := f(pt[:]...)
			if err != nil {
				return nil, err
			}
			if len(c) < 2 {
				return nil, fmt.Errorf("function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			line[i][0], line[i][1] = c[0], c[1]
		}
		return line, nil

	case geom.MultiLineString:
		lines := make(geom.MultiLineString, len(geo))

		for i, line := range geo {
			// getting a geometry inteface back
			linei, err := ApplyToPoints1(geom.LineString(line), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of multiline: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			lines[i] = linev
		}
		return lines, nil

	case geom.Polygon:
		poly := make(geom.Polygon, len(geo))

		for i, line := range geo {
			// getting a geometry inteface back
			linei, err := ApplyToPoints1(geom.LineString(line), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of polygon: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			poly[i] = linev
		}
		return poly, nil

	case geom.MultiPolygon:
		mpoly := make(geom.MultiPolygon, len(geo))

		for i, poly := range geo {
			// getting a geometry inteface back
			polyi, err := ApplyToPoints1(geom.Polygon(poly), f)
			if err != nil {
				return nil, fmt.Errorf("got error converting poly(%v) of multipolygon: %v", i, err)
			}

			// get the value
			polyv, ok := polyi.(geom.Polygon)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			mpoly[i] = polyv
		}
		return mpoly, nil
	}
}

// CloneGeomtry returns a deep clone of the Geometry.
func CloneGeometry1(geometry geom.Geometry) (geom.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("Unknown Geometry: %+v", geometry)

	case geom.Point:
		return geom.Point{geo.X(), geo.Y()}, nil

	case geom.MultiPoint:
		pts := make(geom.MultiPoint, len(geo))
		for i, pt := range geo {
			pts[i] = pt
		}
		return pts, nil

	case geom.LineString:
		line := make(geom.LineString, len(geo))
		for i, pt := range geo {
			line[i] = pt
		}
		return line, nil

	case geom.MultiLineString:
		lines := make(geom.MultiLineString, len(geo))
		for i, line := range geo {
			// getting a geometry interface back
			linei, err := CloneGeometry1(geom.LineString(line))
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of multiline: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			lines[i] = linev
		}
		return lines, nil

	case geom.Polygon:
		// getting a geometry inteface back
		poly := make(geom.Polygon, len(geo))
		for i, line := range geo {
			linei, err := CloneGeometry1(geom.LineString(line))
			if err != nil {
				return nil, fmt.Errorf("got error converting line(%v) of polygon: %v", i, err)
			}

			// get the value
			linev, ok := linei.(geom.LineString)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			poly[i] = linev
		}
		return poly, nil

	case geom.MultiPolygon:
		mpoly := make(geom.MultiPolygon, len(geo))
		for i, poly := range geo {
			// getting a geometry inteface back
			polyi, err := CloneGeometry1(geom.Polygon(poly))
			if err != nil {
				return nil, fmt.Errorf("got error converting polygon(%v) of multipolygon: %v", i, err)
			}

			// get the value
			polyv, ok := polyi.(geom.Polygon)
			if !ok {
				panic("we did not get the conversion we were expecting")
			}

			mpoly[i] = polyv
		}
		return mpoly, nil
	}
}

// ToWebMercator takes a SRID and a geometry encode using that srid, and returns a geometry encoded as a WebMercator.
func ToWebMercator(SRID uint64, geometry geom.Geometry) (geom.Geometry, error) {
	switch SRID {
	default:
		return G{}, fmt.Errorf("Don't know how to convert from %v to %v.", tegola.WebMercator, SRID)
	case tegola.WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.

		return CloneGeometry1(geometry)
	case tegola.WGS84:

		return ApplyToPoints1(geometry, webmercator.PToXY)
	}
}
