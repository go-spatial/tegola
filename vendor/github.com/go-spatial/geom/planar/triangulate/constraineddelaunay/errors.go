package constraineddelaunay

import (
	"fmt"

	"github.com/go-spatial/geom/planar/triangulate/quadedge"
)

type ErrNoMatchingEdgeFound struct {
	T1 *Triangle
	T2 *Triangle
}

func (e ErrNoMatchingEdgeFound) Error() string {
	return fmt.Sprintf("no matching edge found T1: %v T2: %v", e.T1, e.T2)
}

type ErrInvalidVertex struct {
	V quadedge.Vertex
	T *Triangle
}

func (e ErrInvalidVertex) Error() string {
	return fmt.Sprintf("invalid vertex: %v in %v", e.V, e.T)
}
