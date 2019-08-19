package constraineddelaunay

import (
	"fmt"

	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar/triangulate/quadedge"
)

/*
Triangle provides operations on a triangle within a
quadedge.QuadEdgeSubdivision.

This is outside the quadedge package to avoid making changes to the original
JTS port.
*/
type Triangle struct {
	// the triangle referenced is to the right of this edge
	Qe *quadedge.QuadEdge
}

type TriangleByCentroid []Triangle

func (a TriangleByCentroid) Len() int      { return len(a) }
func (a TriangleByCentroid) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a TriangleByCentroid) Less(i, j int) bool {
	return cmp.PointLess(a[i].Centroid().XY(), a[j].Centroid().XY())
}

/*
IntersectsPoint returns true if the vertex intersects the given triangle. This
includes falling on an edge.

If tri is nil a panic will occur.
*/
func (tri *Triangle) IntersectsPoint(v quadedge.Vertex) bool {
	e := tri.Qe

	for i := 0; i < 3; i++ {
		lc := v.Classify(e.Orig(), e.Dest())
		switch lc {
		// return true if v is on the edge
		case quadedge.ORIGIN:
			return true
		case quadedge.DESTINATION:
			return true
		case quadedge.BETWEEN:
			return true
		// return false if v is well outside the triangle
		case quadedge.LEFT:
			return false
		case quadedge.BEHIND:
			return false
		case quadedge.BEYOND:
			return false
		}
		// go to the next edge of the triangle.
		e = e.RNext()
	}

	// if v is to the right of all edges, it is inside the triangle.
	return true
}

func (tri *Triangle) Centroid() quadedge.Vertex {
	v1 := tri.Qe.Orig()
	v2 := tri.Qe.Dest()
	v3 := tri.Qe.RPrev().Dest()

	return v1.Sum(v2).Sum(v3).Divide(3)
}

/*
GetStartEdge returns the 'starting' edge of this triangle. Unless Normalize
has been called the edge is arbitrary.

If tri is nil a panic will occur.
*/
func (tri *Triangle) GetStartEdge() *quadedge.QuadEdge {
	return tri.Qe
}

/*
IsValid returns true if the specified triangle has three sides that connect.

If tri is nil IsValid will return false.
*/
func (tri *Triangle) IsValid() bool {
	if tri == nil {
		return false
	}

	count := 0

	e := tri.Qe
	for {
		e = e.RPrev()
		count++
		if e.Orig().Equals(tri.Qe.Orig()) {
			break
		}
		if count > 3 {
			return false
		}
	}
	return count == 3
}

/*
Normalize returns a triangle that is represented by the QuadEdge that starts
at the "smallest" coordinate.

Because there are multiple ways to represent the same triangle this is
necessary if you want to maintain a set or map of triangles.

If tri is nil a panic will occur.
*/
func (tri *Triangle) Normalize() *Triangle {
	v1 := tri.Qe.Orig()
	v2 := tri.Qe.Dest()
	v3 := tri.Qe.RPrev().Dest()

	c21 := cmp.PointLess(v2, v1)
	c31 := cmp.PointLess(v3, v1)
	c32 := cmp.PointLess(v3, v2)

	if c21 == false && c31 == false {
		// the original is correct
		return tri
	}
	if c31 && c32 {
		return &Triangle{tri.Qe.RNext()}
	}
	return &Triangle{tri.Qe.RPrev()}
}

/*
opposedTriangle returns the triangle opposite to the vertex v.
       +
      /|\
     / | \
    /  |  \
v1 + a | b +
    \  |  /
     \ | /
      \|/
       +

If this method is called on triangle a with v1 as the vertex, the result will be triangle b.

If tri is nil a panic will occur.
*/
func (tri *Triangle) opposedTriangle(v quadedge.Vertex) (*Triangle, error) {
	qe := tri.Qe

	for qe.Orig().Equals(v) == false {

		qe = qe.RNext()

		if qe == tri.Qe {
			return nil, ErrInvalidVertex{v, tri}
		}
	}

	return &Triangle{qe.RNext().RNext().Sym()}, nil
}

/*
opposedVertex returns the vertex opposite to this triangle.
       +
      /|\
     / | \
    /  |  \
v1 + a | b + v2
    \  |  /
     \ | /
      \|/
       +

If this method is called as a.opposedVertex(b), the result will be vertex v2.

If tri is nil a panic will occur.
*/
func (tri *Triangle) opposedVertex(other *Triangle) (quadedge.Vertex, error) {
	ae, err := tri.sharedEdge(other)
	if err != nil {
		return quadedge.Vertex{}, err
	}

	// using the matching edge in triangle a, find the opposed vertex in b.
	return ae.Sym().ONext().Dest(), nil
}

/*
sharedEdge returns the edge that is shared by both a and b. The edge is
returned with triangle a on the left.

       + l
      /|\
     / | \
    /  |  \
   + a | b +
    \  |  /
     \ | /
      \|/
       + r

If this method is called as a.sharedEdge(b), the result will be edge lr.

If tri is nil a panic will occur.
*/
func (tri *Triangle) sharedEdge(other *Triangle) (*quadedge.QuadEdge, error) {
	ae := tri.Qe
	be := other.Qe
	foundMatch := false

	// search for the matching edge between both triangles
	for ai := 0; ai < 3; ai++ {
		for bi := 0; bi < 3; bi++ {
			if ae.Orig().Equals(be.Dest()) && ae.Dest().Equals(be.Orig()) {
				foundMatch = true
				break
			}
			be = be.RNext()
		}

		if foundMatch {
			break
		}
		ae = ae.RNext()
	}

	if foundMatch == false {
		// if there wasn't a matching edge
		return nil, ErrNoMatchingEdgeFound{tri, other}
	}

	// return the matching edge in triangle a
	return ae, nil
}

/*
String returns a string representation of triangle.

If IsValid is false you may get a "triangle" with more or less than three
sides.

If tri is nil a panic will occur.
*/
func (tri *Triangle) String() string {
	str := "["
	e := tri.Qe
	comma := ""
	for {
		str += comma + fmt.Sprintf("%v", e.Orig())
		comma = ","
		e = e.RPrev()
		if e.Orig().Equals(tri.Qe.Orig()) {
			break
		}
	}
	str = str + "]"
	return str
}
