package basic

import (
	"fmt"
	"log"
	"strings"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths/webmercator"
)

// ApplyToPoints applys the given function to each point in the geometry and any sub geometries, return a new transformed geometry.
func ApplyToPoints(geometry tegola.Geometry, f func(coords ...float64) ([]float64, error)) (G, error) {
	switch geo := geometry.(type) {
	default:
		return G{}, fmt.Errorf("Unknown Geometry: %+v", geometry)
	case tegola.Point:
		c, err := f(geo.X(), geo.Y())
		if err != nil {
			return G{}, err
		}
		if len(c) < 2 {
			return G{}, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
		}
		return G{Point{c[0], c[1]}}, nil
	case tegola.Point3:
		c, err := f(geo.X(), geo.Y(), geo.Z())
		if err != nil {
			return G{}, err
		}
		if len(c) < 3 {
			return G{}, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 3", len(c))
		}
		return G{Point3{c[0], c[1], c[2]}}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for _, pt := range geo.Points() {
			c, err := f(pt.X(), pt.Y())
			if err != nil {
				return G{}, err
			}
			if len(c) < 2 {
				return G{}, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			pts = append(pts, Point{c[0], c[1]})
		}
		return G{pts}, nil
	case tegola.LineString:
		var line Line
		for _, ptGeo := range geo.Subpoints() {
			c, err := f(ptGeo.X(), ptGeo.Y())
			if err != nil {
				return G{}, err
			}
			if len(c) < 2 {
				return G{}, fmt.Errorf("Function did not return minimum number of coordinates got %v expected 2", len(c))
			}
			line = append(line, Point{c[0], c[1]})
		}
		return G{line}, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geoLine, err := ApplyToPoints(lineGeo, f)
			if err != nil {
				return G{}, fmt.Errorf("Got error converting line(%v) of multiline: %v", i, err)
			}
			if !geoLine.IsLine() {
				panic("We did not get the conversion we were expecting")
			}
			line = append(line, geoLine.AsLine())
		}
		return G{line}, nil
	case tegola.Polygon:
		var poly Polygon
		for i, line := range geo.Sublines() {
			geoLine, err := ApplyToPoints(line, f)
			if err != nil {
				return G{}, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			poly = append(poly, geoLine.AsLine())
		}
		return G{poly}, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geoPoly, err := ApplyToPoints(poly, f)
			if err != nil {
				return G{}, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			mpoly = append(mpoly, geoPoly.AsPolygon())
		}
		return G{mpoly}, nil
	}
}

// CloneGeomtry returns a deep clone of the Geometry.
func CloneGeometry(geometry tegola.Geometry) (G, error) {
	switch geo := geometry.(type) {
	default:
		return G{}, fmt.Errorf("Unknown Geometry: %+v", geometry)
	case tegola.Point:
		return G{Point{geo.X(), geo.Y()}}, nil
	case tegola.Point3:
		return G{Point3{geo.X(), geo.Y(), geo.Z()}}, nil
	case tegola.MultiPoint:
		var pts MultiPoint
		for _, pt := range geo.Points() {
			pts = append(pts, Point{pt.X(), pt.Y()})
		}
		return G{pts}, nil
	case tegola.LineString:
		var line Line
		for _, ptGeo := range geo.Subpoints() {
			line = append(line, Point{ptGeo.X(), ptGeo.Y()})
		}
		return G{line}, nil
	case tegola.MultiLine:
		var line MultiLine
		for i, lineGeo := range geo.Lines() {
			geom, err := CloneGeometry(lineGeo)
			if err != nil {
				return G{}, fmt.Errorf("Got error converting line(%v) of multiline: %v", i, err)
			}
			line = append(line, geom.AsLine())
		}
		return G{line}, nil
	case tegola.Polygon:
		var poly Polygon
		for i, line := range geo.Sublines() {
			log.Printf("Cloning %T:%[1]t\n", line)
			geom, err := CloneGeometry(line)
			if err != nil {
				return G{}, fmt.Errorf("Got error converting line(%v) of polygon: %v", i, err)
			}
			poly = append(poly, geom.AsLine())
		}
		return G{poly}, nil
	case tegola.MultiPolygon:
		var mpoly MultiPolygon
		for i, poly := range geo.Polygons() {
			geom, err := CloneGeometry(poly)
			if err != nil {
				return G{}, fmt.Errorf("Got error converting poly(%v) of multipolygon: %v", i, err)
			}
			mpoly = append(mpoly, geom.AsPolygon())
		}
		return G{mpoly}, nil
	}
}

// ToWebMercator takes a SRID and a geometry encode using that srid, and returns a geometry encoded as a WebMercator.
func ToWebMercator(SRID int, geometry tegola.Geometry) (G, error) {
	switch SRID {
	default:
		return G{}, fmt.Errorf("Don't know how to convert from %v to %v.", tegola.WebMercator, SRID)
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
func FromWebMercator(SRID int, geometry tegola.Geometry) (G, error) {
	switch SRID {
	default:
		return G{}, fmt.Errorf("Don't know how to convert from %v to %v.", SRID, tegola.WebMercator)
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
	return nil, fmt.Errorf("Unknown type")
}
