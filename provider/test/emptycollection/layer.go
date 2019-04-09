package emptycollection

import "github.com/go-spatial/geom"

type layer struct {
	name     string
	geomType geom.Geometry
	srid     uint64
}

func (l layer) Name() string {
	return l.name
}

func (l layer) GeomType() geom.Geometry {
	return l.geomType
}

func (l layer) SRID() uint64 {
	return l.srid
}
