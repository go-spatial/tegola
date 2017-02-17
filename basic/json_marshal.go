package basic

import (
	"encoding/json"
	"fmt"
)

/*
This file contains helper functions for json marshaling.
*/

type basicGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates,omitempty"`
	Geometries  []basicGeometry `json:"geometries,omitempty"`
}

func point(pts []float64) (Point, error) {
	if len(pts) < 2 {
		return Point{}, fmt.Errorf("Not enough points for a Point")
	}
	return Point{pts[0], pts[1]}, nil
}
func point3(pts []float64) (Point3, error) {
	if len(pts) < 3 {
		return Point3{}, fmt.Errorf("Not enough points for a Point3")
	}
	return Point3{pts[0], pts[1], pts[2]}, nil
}
func pointSlice(pts [][]float64) (mp []Point, err error) {
	for _, p := range pts {
		pt, err := point(p)
		if err != nil {
			return nil, err
		}
		mp = append(mp, pt)
	}
	return mp, nil
}

func unmarshalBasicGeometry(bgeo basicGeometry) (geo Geometry, err error) {
	switch bgeo.Type {
	case "Point":
		var pts []float64
		if err = json.Unmarshal(bgeo.Coordinates, &pts); err != nil {
			return nil, err
		}
		return point(pts)
	case "Point3":
		var pts []float64
		if err = json.Unmarshal(bgeo.Coordinates, &pts); err != nil {
			return nil, err
		}
		return point3(pts)
	case "MultiPoint":
		var pts [][]float64
		if err = json.Unmarshal(bgeo.Coordinates, &pts); err != nil {
			return nil, err
		}
		mp, err := pointSlice(pts)
		return MultiPoint(mp), err
	case "MultiPoint3":
		var mp MultiPoint3
		var pts [][]float64
		if err = json.Unmarshal(bgeo.Coordinates, &pts); err != nil {
			return nil, err
		}
		for _, p := range pts {
			pt, err := point3(p)
			if err != nil {
				return nil, err
			}
			mp = append(mp, pt)
		}
		return mp, nil

	case "LineString":
		var pts [][]float64
		if err = json.Unmarshal(bgeo.Coordinates, &pts); err != nil {
			return nil, err
		}
		mp, err := pointSlice(pts)
		return Line(mp), err

	case "MultiLineString":
		var ml MultiLine
		var lines [][][]float64
		if err = json.Unmarshal(bgeo.Coordinates, &lines); err != nil {
			return nil, err
		}
		for _, lpts := range lines {
			mp, err := pointSlice(lpts)
			if err != nil {
				return nil, err
			}
			ml = append(ml, Line(mp))
		}
		return ml, nil

	case "Polygon":
		var p Polygon
		var lines [][][]float64
		if err = json.Unmarshal(bgeo.Coordinates, &lines); err != nil {
			return nil, err
		}
		for _, lpts := range lines {
			mp, err := pointSlice(lpts)
			if err != nil {
				return nil, err
			}
			p = append(p, Line(mp))
		}
		return p, nil

	case "MultiPolygon":
		var mpoly MultiPolygon
		var polygons [][][][]float64
		if err = json.Unmarshal(bgeo.Coordinates, &polygons); err != nil {
			return nil, err
		}
		for _, lines := range polygons {
			var p Polygon
			for _, lpts := range lines {
				mp, err := pointSlice(lpts)
				if err != nil {
					return nil, err
				}
				p = append(p, Line(mp))
			}
			mpoly = append(mpoly, p)
		}
		return mpoly, nil

	case "GeometeryCollection":
		var c Collection
		for _, basicgeo := range bgeo.Geometries {

			geo, err := unmarshalBasicGeometry(basicgeo)
			if err != nil {
				return nil, err
			}
			c = append(c, geo)
		}
		return c, nil

	}
	return nil, fmt.Errorf("Unknown Type (%v).", bgeo.Type)

}

func UnmarshalJSON(data []byte) (geo Geometry, err error) {
	var bgeo basicGeometry
	if err = json.Unmarshal(data, &bgeo); err != nil {
		return nil, err
	}
	return unmarshalBasicGeometry(bgeo)
}

/*=========================  BASIC TYPES ======================================*/

func jsonTemplate(name, coords string) []byte {
	return []byte(`{"type":"` + name + `","coordinates":` + coords + `}`)
}

// MarshalJSON
func (p Point) internalMarshalJSON() string {
	return fmt.Sprintf(`[%v,%v]`, p[0], p[1])
}
func (p Point) MarshalJSON() ([]byte, error) {
	return jsonTemplate("Point", p.internalMarshalJSON()), nil
}
func (p Point3) internalMarshalJSON() string {
	return fmt.Sprintf(`[%v,%v,%v]`, p[0], p[1], p[2])
}
func (p Point3) MarshalJSON() ([]byte, error) {
	return jsonTemplate("Point3", p.internalMarshalJSON()), nil
}

func (p MultiPoint) internalMarshalJSON() string {
	s := "["
	for i, pt := range p {
		if i != 0 {
			s += ","
		}
		s += pt.internalMarshalJSON()
	}
	s += `]`
	return s
}
func (p MultiPoint) MarshalJSON() ([]byte, error) {
	return jsonTemplate("MultiPoint", p.internalMarshalJSON()), nil
}
func (p MultiPoint3) internalMarshalJSON() string {
	s := "["
	for i, pt := range p {
		if i != 0 {
			s += ","
		}
		s += pt.internalMarshalJSON()
	}
	s += `]`
	return s
}
func (p MultiPoint3) MarshalJSON() ([]byte, error) {
	return jsonTemplate("MultiPoint3", p.internalMarshalJSON()), nil
}

func (l Line) internalMarshalJSON() string {
	s := "["
	for i, pt := range l {
		if i != 0 {
			s += ","
		}
		s += pt.internalMarshalJSON()
	}
	s += `]`
	return s
}

func (l Line) MarshalJSON() ([]byte, error) {
	return jsonTemplate("LineString", l.internalMarshalJSON()), nil
}
func (l MultiLine) internalMarshalJSON() string {
	s := "["
	for i, line := range l {
		if i != 0 {
			s += ","
		}
		s += line.internalMarshalJSON()
	}
	s += `]`
	return s
}
func (l MultiLine) MarshalJSON() ([]byte, error) {
	return jsonTemplate("MultiLineString", l.internalMarshalJSON()), nil
}
func (p Polygon) internalMarshalJSON() string {
	s := "["
	for i, line := range p {
		if i != 0 {
			s += ","
		}
		s += line.internalMarshalJSON()
	}
	s += `]`
	return s
}
func (p Polygon) MarshalJSON() ([]byte, error) {
	return jsonTemplate("Polygon", p.internalMarshalJSON()), nil
}
func (p MultiPolygon) internalMarshalJSON() string {
	s := "["
	for i, line := range p {
		if i != 0 {
			s += ","
		}
		s += line.internalMarshalJSON()
	}
	s += `]`
	return s
}
func (p MultiPolygon) MarshalJSON() ([]byte, error) {
	return jsonTemplate("MultiPolygon", p.internalMarshalJSON()), nil
}
func (c Collection) MarshalJSON() ([]byte, error) {
	js := []byte(`{"type":"GeometryCollection","geometries":[`)
	for i, g := range c {
		if i != 0 {
			js = append(js, ',')
		}
		v, ok := g.(json.Marshaler)
		if !ok {
			continue
		}
		b, e := v.MarshalJSON()
		if e != nil {
			return nil, e
		}
		js = append(js, b...)
	}
	js = append(js, ']', '}')
	return js, nil
}
