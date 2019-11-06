package proj

import (
	"fmt"
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/proj/webmercator"
)

var (
	WebMercatorBounds = &webmercator.Extent
	WGS84Bounds       = &geom.Extent{-180.0, -85.0511, 180.0, 85.0511}
)

const (
	WebMercator = webmercator.SRID
	WGS84       = 4326
	Deg2Rad     = math.Pi / 180
	Rad2Deg     = 180 / math.Pi
	PiDiv2      = math.Pi / 2.0
	PiDiv4      = math.Pi / 4.0
)

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints(geometry geom.Geometry, f func(coords ...float64) ([]float64, error)) (geom.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("unknown Geometry: %T", geometry)

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
			// getting a geometry interface back
			linei, err := ApplyToPoints(geom.LineString(line), f)
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
			linei, err := ApplyToPoints(geom.LineString(line), f)
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
			polyi, err := ApplyToPoints(geom.Polygon(poly), f)
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
func CloneGeometry(geometry geom.Geometry) (geom.Geometry, error) {
	switch geo := geometry.(type) {
	default:
		return nil, fmt.Errorf("unknown Geometry: %T", geometry)

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
			linei, err := CloneGeometry(geom.LineString(line))
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
			linei, err := CloneGeometry(geom.LineString(line))
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
			polyi, err := CloneGeometry(geom.Polygon(poly))
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
		return nil, fmt.Errorf("don't know how to convert from %v to %v.", WebMercator, SRID)
	case WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.

		return CloneGeometry(geometry)
	case WGS84:

		return ApplyToPoints(geometry, webmercator.PToXY)
	}
}

// FromWebMercator takes a geometry encoded with WebMercator, and returns a Geometry encodes to the given srid.
func FromWebMercator(SRID uint64, geometry geom.Geometry) (geom.Geometry, error) {
	switch SRID {
	default:
		return nil, fmt.Errorf("don't know how to convert from %v to %v.", SRID, WebMercator)
	case WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.
		return CloneGeometry(geometry)
	case WGS84:
		return ApplyToPoints(geometry, webmercator.PToLonLat)
	}
}
