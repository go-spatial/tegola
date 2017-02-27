package basic

import "fmt"

// G is used to pass back a generic Geometry type. It will contains functions to do basic conversions.
type G struct {
	Geometry
}

func (g G) getbase() G {
	rg := g
	bg, ok := g.Geometry.(G)
	for ok {
		rg = bg
		bg, ok = g.Geometry.(G)
	}
	return rg

}

func (g G) IsLine() bool {
	_, ok := g.getbase().Geometry.(Line)
	return ok
}

func (g G) AsLine() Line {
	bg := g.getbase()
	l, ok := bg.Geometry.(Line)
	if !ok {
		panic(fmt.Sprintf("Geo is not a Line! : %T", bg.Geometry))
	}
	return l
}

func (g G) IsPolygon() bool {
	_, ok := g.getbase().Geometry.(Polygon)
	return ok
}

func (g G) AsPolygon() Polygon {
	bg := g.getbase()
	p, ok := bg.Geometry.(Polygon)
	if !ok {
		panic(fmt.Sprintf("Geo is not a Polygon! : %T", bg.Geometry))
	}
	return p
}
func (g G) AsMultiPolygon() MultiPolygon {
	bg := g.getbase()
	p, ok := bg.Geometry.(MultiPolygon)
	if !ok {
		panic(fmt.Sprintf("Geo is not a MultiPolygon! : %T", bg.Geometry))
	}
	return p
}

func (g G) IsPoint() bool {
	_, ok := g.getbase().Geometry.(Point)
	return ok
}

func (g G) AsPoint() Point {
	bg := g.getbase()
	p, ok := bg.Geometry.(Point)
	if !ok {
		panic(fmt.Sprintf("Geo is not a Point! : %T", bg.Geometry))
	}
	return p
}
