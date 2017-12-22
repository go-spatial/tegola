package debug

import "github.com/terranodo/tegola"

type Layer struct {
	name     string
	geomType tegola.Geometry
	srid     int
}

func (l Layer) Name() string {
	return l.name
}

func (l Layer) GeomType() tegola.Geometry {
	return l.geomType
}

func (l Layer) SRID() int {
	return l.srid
}
