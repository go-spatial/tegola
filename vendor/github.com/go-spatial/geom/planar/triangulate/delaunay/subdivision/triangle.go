package subdivision

import (
	"errors"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/quadedge"
)

type Triangle struct {
	*quadedge.Edge
}

func NewTriangle(e *quadedge.Edge) Triangle     { return Triangle{e} }
func (t Triangle) StartingEdge() *quadedge.Edge { return t.Edge }
func (t Triangle) IntersectsPoint(pt geom.Point) bool {
	e := t.StartingEdge()
	if e == nil {
		return false
	}

	for i := 0; i < 3; i++ {
		switch quadedge.Classify(pt, *e.Orig(), *e.Dest()) {

		// return true if v is on the edge
		case quadedge.ORIGIN, quadedge.DESTINATION, quadedge.BETWEEN:
			return true
			// return false if v is well outside the triangle
		case quadedge.LEFT, quadedge.BEHIND, quadedge.BEYOND:
			return false
		}
		e = e.RNext()
	}
	// if v is to the right of all edges, it is inside the triangle.
	return true
}

// OppositeVertex returns the vertex opposite to this triangle.
//        +
//       /|\
//      / | \
//     /  |  \
// v1 + a | b + v2
//     \  |  /
//      \ | /
//       \|/
//        +
//
// If this method is called as a.opposedVertex(b), the result will be vertex v2.
func (t Triangle) OppositeVertex(other Triangle) *geom.Point {
	ae := t.SharedEdge(other)
	if ae == nil {
		return nil
	}
	return ae.Sym().ONext().Dest()
}

// OppositeTriangle returns the triangle opposite to the vertex v
//        +
//       /|\
//      / | \
//     /  |  \
// v1 + a | b +
//     \  |  /
//      \ | /
//       \|/
//        +
//
// If this method is called on triangle a with v1 as the vertex, the result will be triangle b.
func (t Triangle) OppositeTriangle(p geom.Point) (*Triangle, error) {
	start := t.StartingEdge()
	edge := start
	for !cmp.GeomPointEqual(*edge.Orig(), p) {
		edge = edge.RNext()
		if edge == start {
			return nil, errors.New("invalid vertex")
		}
	}
	return &Triangle{edge.RNext().RNext().Sym()}, nil
}

// SharedEdge returns the edge that is shared by both a and b. The edge is
// returned with triangle a on the left.
//
//        + l
//       /|\
//      / | \
//     /  |  \
//    + a | b +
//     \  |  /
//      \ | /
//       \|/
//        + r
//
// If this method is called as a.sharedEdge(b), the result will be edge lr.
//
func (t Triangle) SharedEdge(other Triangle) *quadedge.Edge {
	ae := t.StartingEdge()
	be := other.StartingEdge()

	for ai := 0; ai < 3; ai, ae = ai+1, ae.RNext() {
		for bi := 0; bi < 3; bi, be = bi+1, be.RNext() {
			if cmp.GeomPointEqual(*ae.Orig(), *be.Dest()) &&
				cmp.GeomPointEqual(*be.Orig(), *ae.Dest()) {
				return ae
			}
		}
	}
	return nil
}
