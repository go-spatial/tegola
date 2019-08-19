package quadedge

import (
	"log"

	"github.com/go-spatial/geom/planar"

	"github.com/go-spatial/geom"
)

// Splice operator affects the two edge rings around the origin of a and b,
// and, independently, the two edge rings around the left faces of a and b.
// In each case, (i) if the two rings are distinct, Splace will combine
// them into one; (ii) if the two are the same ring, Splice will break it
// into two separate pieces.
// Thus, Splice can be used both to attach the two edges together, and
// to break them apart. See Guibas and Stolfi (1985) p.96 for more details
// and illustrations.
func Splice(a, b *Edge) {
	if a == nil || b == nil {
		return
	}
	alpha := a.ONext().Rot()
	beta := b.ONext().Rot()

	t1 := b.ONext()
	t2 := a.ONext()
	t3 := beta.ONext()
	t4 := alpha.ONext()

	a.next = t1
	b.next = t2
	alpha.next = t3
	beta.next = t4
}

// Connect Add a new edge e connection the destination of a to the
// origin of b, in such a way that all three have the same
// left face after the connection is complete.
// Additionally, the data pointers of the new edge are set.
func Connect(a, b *Edge) *Edge {
	e := New()
	Splice(e, a.LNext())
	Splice(e.Sym(), b)
	e.EndPoints(a.Dest(), b.Orig())
	return e
}

// Swap Essentially turns edge e counterclockwase inside its enclosing
// quadrilateral. The data pointers are modified accordingly.
func Swap(e *Edge) {
	a := e.OPrev()
	b := e.Sym().OPrev()
	Splice(e, a)
	Splice(e.Sym(), b)
	Splice(e, a.LNext())
	Splice(e.Sym(), b.LNext())
	e.EndPoints(a.Dest(), b.Dest())
}

// Delete will remove the edge from the ring
func Delete(e *Edge) {
	if e == nil {
		return
	}
	if debug {
		log.Printf("Deleting edge %p", e)
	}
	Splice(e, e.OPrev())
	Splice(e.Sym(), e.Sym().OPrev())
}

// OnEdge determines if the point x is on the edge e.
func OnEdge(pt geom.Point, e *Edge) bool {
	org := e.Orig()
	if org == nil {
		return false
	}
	dst := e.Dest()
	if dst == nil {
		return false
	}
	l := geom.Line{*org, *dst}
	return planar.IsPointOnLineSegment(pt, l)
}

// RightOfOrOn indicate if the point is on the line created by the Edge or right of the edge
func RightOfOrOn(x geom.Point, e *Edge) bool {
	if planar.IsPointOnLine([2]float64(x), [2]float64(*e.Orig()), [2]float64(*e.Dest())) {
		return true
	}
	return RightOf(x, e)
}

// RightOf indicates if the point is right of the Edge
func RightOf(x geom.Point, e *Edge) bool {
	org := e.Orig()
	if org == nil {
		return false
	}
	dst := e.Dest()
	if dst == nil {
		return false
	}
	return planar.IsCCW(x, *dst, *org)
}

// LeftOfOrOn indicate if the point is on the line created by the Edge or right of the edge
func LeftOfOrOn(x geom.Point, e *Edge) bool {
	if planar.IsPointOnLine([2]float64(x), [2]float64(*e.Orig()), [2]float64(*e.Dest())) {
		return true
	}
	return LeftOf(x, e)
}

// LeftOf indicates if the point is left of the Edge
func LeftOf(x geom.Point, e *Edge) bool {
	org := e.Orig()
	if org == nil {
		return false
	}
	dst := e.Dest()
	if dst == nil {
		return false
	}
	return planar.IsCCW(x, *org, *dst)
}
