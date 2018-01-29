package debug

import "github.com/terranodo/tegola/geom"

type Layer struct {
	name     string
	geomType geom.Geometry
	srid     int
}

func (l Layer) Name() string {
	return l.name
}

func (l Layer) GeomType() geom.Geometry {
	return l.geomType
}

func (l Layer) SRID() int {
	return l.srid
}
