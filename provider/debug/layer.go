package debug

import "github.com/go-spatial/geom"

type Layer struct {
	name     string
	geomType geom.Geometry
	srid     uint64
}

func (l Layer) Name() string {
	return l.name
}

func (l Layer) GeomType() geom.Geometry {
	return l.geomType
}

func (l Layer) SRID() uint64 {
	return l.srid
}
