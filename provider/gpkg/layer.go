package gpkg

import (
	"github.com/go-spatial/geom"
)

type Layer struct {
	name            string
	tablename       string
	features        []string
	tagFieldnames   []string
	idFieldname     string
	geomFieldname   string
	tstartFieldname string
	tendFieldname   string
	geomType        geom.Geometry
	srid            uint64
	bbox            geom.Extent
	sql             string
	mtag            string
}

func (l Layer) Name() string             { return l.name }
func (l Layer) GeomType() geom.Geometry  { return l.geomType }
func (l Layer) SRID() uint64             { return l.srid }
func (l Layer) ModificationTag() *string { return &l.mtag }
