package basic

// G is used to pass back a generic Geometry type. It will contains functions to do basic conversions.
type G struct {
	Geometry
}

func (g G) IsLine() bool {
	_, ok := g.Geometry.(Line)
	return ok
}

func (g G) AsLine() Line {
	l, ok := g.Geometry.(Line)
	if !ok {
		panic("Geo is not a line!")
	}
	return l
}

func (g G) IsPolygon() bool {
	_, ok := g.Geometry.(Polygon)
	return ok
}

func (g G) AsPolygon() Polygon {
	p, ok := g.Geometry.(Polygon)
	if !ok {
		panic("Geo is not a Polygon!")
	}
	return p
}
func (g G) AsMultiPolygon() MultiPolygon {
	p, ok := g.Geometry.(MultiPolygon)
	if !ok {
		panic("Geo is not a Polygon!")
	}
	return p
}

func (g G) IsPoint() bool {
	_, ok := g.Geometry.(Point)
	return ok
}

func (g G) AsPoint() Point {
	p, ok := g.Geometry.(Point)
	if !ok {
		panic("Geo is not a Point!")
	}
	return p
}
