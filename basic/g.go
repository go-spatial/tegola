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

func (g G) IsMultiPolygon() bool {
	_, ok := g.getbase().Geometry.(MultiPolygon)
	return ok
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
	// While a Point3 can be cast to a Point, we don't want a false positive
	_, ok := g.getbase().Geometry.(Point3)
	if ok {
		return false
	}

	_, ok = g.getbase().Geometry.(Point)
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

func (g G) IsPoint3() bool {
	_, ok := g.getbase().Geometry.(Point3)
	return ok
}

func (g G) AsPoint3() Point3 {
	p, ok := g.getbase().Geometry.(Point3)
	if !ok {
		panic(fmt.Sprintf("Geo is not a Point3! : %T", g.getbase().Geometry))
	}
	return p
}

func (g G) IsMultiPoint() bool {
	// While a MultiPoint3 can be cast to a MultiPoint, we don't want a false positive
	_, ok := g.getbase().Geometry.(MultiPoint3)
	if ok {
		return false
	}
	_, ok = g.getbase().Geometry.(MultiPoint)
	return ok
}

func (g G) AsMultiPoint() MultiPoint {
	mp, ok := g.getbase().Geometry.(MultiPoint)
	if !ok {
		panic(fmt.Sprintf("Geo is not a MultiPoint! : %T", g.getbase().Geometry))
	}
	return mp
}

func (g G) IsMultiPoint3() bool {
	_, ok := g.getbase().Geometry.(MultiPoint3)
	return ok
}

func (g G) AsMultiPoint3() MultiPoint3 {
	mp, ok := g.getbase().Geometry.(MultiPoint3)
	if !ok {
		panic(fmt.Sprintf("Geo is not a MultiPoint3! : %T", g.getbase().Geometry))
	}
	return mp
}

func (g G) IsMultiLine() bool {
	_, ok := g.getbase().Geometry.(MultiLine)
	return ok
}

func (g G) AsMultiLine() MultiLine {
	ml, ok := g.getbase().Geometry.(MultiLine)
	if !ok {
		panic(fmt.Sprintf("Geo is not a MultiLine! : %T", g.getbase().Geometry))
	}
	return ml
}
