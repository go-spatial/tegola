package gpkg

import (
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/maths/points"
)

type Layer struct {
	name          string
	tablename     string
	features      []string
	tagFieldnames []string
	idFieldname   string
	geomFieldname string
	geomType      geom.Geometry
	srid          uint64
	// Bounding box containing all features in the layer: [minX, minY, maxX, maxY]
	bbox points.BoundingBox
	sql  string
}

func (l Layer) Name() string            { return l.name }
func (l Layer) GeomType() geom.Geometry { return l.geomType }
func (l Layer) SRID() uint64            { return l.srid }
func (l Layer) BBox() [4]float64        { return l.bbox }
