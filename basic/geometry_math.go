package basic

import (
	"fmt"
	"strings"

	"errors"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths/webmercator"
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
		return nil, fmt.Errorf("don't know how to convert from %v to %v.", tegola.WebMercator, SRID)
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
func FromWebMercator(SRID uint64, geometry geom.Geometry) (geom.Geometry, error) {
	switch SRID {
	default:
		return nil, fmt.Errorf("don't know how to convert from %v to %v.", SRID, tegola.WebMercator)
	case tegola.WebMercator:
		// Instead of just returning the geometry, we are cloning it so that the user of the API can rely
		// on the result to alway be a copy. Instead of being a reference in the on instance that it's already
		// in the same SRID.
		return CloneGeometry(geometry)
	case tegola.WGS84:
		return ApplyToPoints(geometry, webmercator.PToLonLat)
	}
}

func interfaceAsFloatslice(v interface{}) (vals []float64, err error) {
	vs, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("Incorrect value type looking for float64 slice, not %t.", v)
	}
	for _, iv := range vs {
		vv, ok := iv.(float64)
		if !ok {
			return nil, fmt.Errorf("Incorrect value type looking for float64 slice, not %t.", v)
		}
		vals = append(vals, vv)
	}
	return vals, nil
}

func mapIsOfType(v map[string]interface{}, wants ...string) (string, error) {
	typ, ok := v["type"].(string)
	if !ok {
		return "", fmt.Errorf("Was not able to convert type to string.")
	}
	for _, want := range wants {
		if typ == want {
			return typ, nil
		}
	}
	return "", fmt.Errorf("Expected all subtypes to be one of type (%v), not %v", strings.Join(wants, ","), v["type"])
}

func interfaceAsLine(v interface{}) (Line, error) {
	vals, err := interfaceAsFloatslice(v)
	if err != nil {
		return nil, fmt.Errorf("Incorrect values for line type: %v", err)
	}
	return NewLine(vals...), nil
}

func interfaceAsPoint(v interface{}) (Point, error) {
	vals, err := interfaceAsFloatslice(v)
	if err != nil {
		return Point{}, fmt.Errorf("Incorrect values for point type: %v", err)
	}
	return Point{vals[0], vals[1]}, nil
}

func interfaceAsPoint3(v interface{}) (Point3, error) {
	vals, err := interfaceAsFloatslice(v)
	if err != nil {
		return Point3{}, fmt.Errorf("Incorrect values for point3 type: %v", err)
	}
	return Point3{vals[0], vals[1], vals[2]}, nil
}

func forEachMapInSlice(v interface{}, do func(typ string, v interface{}) error, wants ...string) error {
	vals, ok := v.([]interface{})
	if !ok {
		return fmt.Errorf("Expected values to be []interface{}: ")
	}
	for i, iv := range vals {

		v, ok := iv.(map[string]interface{})
		if !ok {
			return fmt.Errorf("Expected v[%v] to be map[string]interface{}: ", i)
		}
		typ, err := mapIsOfType(v, wants...)
		if err != nil {
			return err
		}
		if err = do(typ, v["value"]); err != nil {
			return err
		}
	}
	return nil
}

func interfaceAsPolygon(v interface{}) (Polygon, error) {
	var p Polygon
	err := forEachMapInSlice(v, func(_ string, v interface{}) error {
		l, err := interfaceAsLine(v)
		if err != nil {
			return err
		}
		p = append(p, l)
		return nil
	}, "linestring")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func MapAsGeometry(m map[string]interface{}) (geo Geometry, err error) {
	typ, err := mapIsOfType(m, "point", "point3", "linestring", "polygon", "multipolygon", "multipoint", "multiline")
	if err != nil {
		return nil, err
	}
	switch typ {
	case "point":
		return interfaceAsPoint(m["value"])
	case "point3":
		return interfaceAsPoint3(m["value"])

	case "linestring":
		return interfaceAsLine(m["value"])
	case "polygon":
		fmt.Println("Working on Polygon:")
		return interfaceAsPolygon(m["value"])
	case "multipolygon":
		fmt.Println("Working on MPolygon:")
		var mp MultiPolygon
		err := forEachMapInSlice(m["value"], func(_ string, v interface{}) error {
			p, err := interfaceAsPolygon(v)
			if err != nil {
				return err
			}
			mp = append(mp, p)
			return nil
		}, "polygon")
		if err != nil {
			return nil, err
		}
		return mp, nil
	case "multipoint":
		var mp MultiPoint
		err := forEachMapInSlice(m["value"], func(_ string, v interface{}) error {
			p, err := interfaceAsPoint(v)
			if err != nil {
				return err
			}
			mp = append(mp, p)
			return nil
		}, "point")
		if err != nil {
			return nil, err
		}
		return mp, nil
	case "multiline":
		var ml MultiLine
		err := forEachMapInSlice(m["value"], func(_ string, v interface{}) error {
			l, err := interfaceAsLine(v)
			if err != nil {
				return err
			}
			ml = append(ml, l)
			return nil
		}, "linestring")
		if err != nil {
			return nil, err
		}
		return ml, nil
	}
	return nil, errors.New("Unknown type")
}
