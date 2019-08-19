package qetriangulate

import (
	"context"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar/triangulate/gdey/quadedge/subdivision"
)

// Triangulator represents a delaunay triangulation
type Triangulator struct {
	points [][2]float64
}

// New creates new Triangulator that can be use to create a delaunay triangulation based
// on the provided points
func New(pts ...[2]float64) *Triangulator {
	return &Triangulator{
		points: pts,
	}
}

// Triangles returns the Triangles from the triangulation. If includeFrame is true the frame triangle will be included
func (t *Triangulator) Triangles(ctx context.Context, includeFrame bool) ([]geom.Triangle, error) {
	sd := subdivision.NewForPoints(ctx, t.points)
	tris, err := sd.Triangles(includeFrame)
	if err != nil {
		return nil, err
	}
	triangles := make([]geom.Triangle, len(tris))
	for i := range tris {
		triangles[i] = geom.Triangle{tris[i][0], tris[i][1], tris[i][2]}
	}
	return triangles, nil
}
