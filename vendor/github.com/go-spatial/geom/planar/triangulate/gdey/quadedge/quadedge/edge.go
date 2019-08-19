package quadedge

import (
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/encoding/wkt"
)

const (
	precision = 6
)

// Edge describes a directional edge in a quadedge
type Edge struct {
	num  int
	next *Edge
	qe   *QuadEdge
	v    *geom.Point
}

// New will return a new edge that is part of an QuadEdge
func New() *Edge {
	ql := NewQEdge()
	return &ql.e[0]
}

// NewWithEndPoints creates a new edge with the given end points
func NewWithEndPoints(a, b *geom.Point) *Edge {
	e := New()
	e.EndPoints(a, b)
	return e
}

// QEdge returns the quadedge this edge is part of
func (e *Edge) QEdge() *QuadEdge {
	if e == nil {
		return nil
	}
	return e.qe
}

// Orig returns the origin end point
func (e *Edge) Orig() *geom.Point {
	if e == nil {
		return nil
	}
	return e.v
}

// Dest returns the destination end point
func (e *Edge) Dest() *geom.Point {
	return e.Sym().Orig()
}

// EndPoints sets the end points of the Edge
func (e *Edge) EndPoints(org, dest *geom.Point) {
	e.v = org
	e.Sym().v = dest
}

// AsLine returns the Edge as a geom.Line
func (e *Edge) AsLine() geom.Line {
	porig, pdest := e.Orig(), e.Dest()
	orig, dest := geom.EmptyPoint, geom.EmptyPoint
	if porig != nil {
		orig = *porig
	}
	if pdest != nil {
		dest = *pdest
	}
	return geom.Line{[2]float64(orig), [2]float64(dest)}
}

/******** Edge Algebra *********************************************************/

// Rot returns the dual of the current edge, directed from its right
// to its left.
func (e *Edge) Rot() *Edge {
	if e == nil {
		return nil
	}
	if e.num == 3 {
		return &(e.qe.e[0])
	}
	return &(e.qe.e[e.num+1])
}

// InvRot returns the dual of the current edge, directed from its left
// to its right.
func (e *Edge) InvRot() *Edge {
	if e == nil {
		return nil
	}
	if e.num == 0 {
		return &(e.qe.e[3])
	}
	return &(e.qe.e[e.num-1])
}

// Sym returns the edge from the destination to the origin of the current edge.
func (e *Edge) Sym() *Edge {
	if e == nil {
		return nil
	}
	if e.num < 2 {
		return &(e.qe.e[e.num+2])
	}
	return &(e.qe.e[e.num-2])
}

// ONext returns the next ccw edge around (from) the origin of the current edge
func (e *Edge) ONext() *Edge {
	if e == nil {
		return nil
	}
	return e.next
}

// OPrev returns the next cw edge around (from) the origin of the currect edge.
func (e *Edge) OPrev() *Edge {
	return e.Rot().ONext().Rot()
}

// DNext returns the next ccw edge around (into) the destination of the current edge.
func (e *Edge) DNext() *Edge {
	return e.Sym().ONext().Sym()
}

// DPrev returns the next cw edge around (into) the destination of the current edge.
func (e *Edge) DPrev() *Edge {
	return e.InvRot().ONext().InvRot()
}

// LNext returns the ccw edge around the left face following the current edge.
func (e *Edge) LNext() *Edge {
	return e.InvRot().ONext().Rot()
}

// LPrev returns the ccw edge around the left face before the current edge.
func (e *Edge) LPrev() *Edge {
	return e.ONext().Sym()
}

// RNext returns the edge around the right face ccw following the current edge.
func (e *Edge) RNext() *Edge {
	return e.Rot().ONext().InvRot()
}

// RPrev returns the edge around the right face ccw before the current edge.
func (e *Edge) RPrev() *Edge {
	return e.Sym().ONext()
}

/*****************************************************************************/
/*         Convenience functions to find edges                                 */
/*****************************************************************************/

// FindONextDest will look for and return a ccw edge the given dest point, if it
// exists.
func (e *Edge) FindONextDest(dest geom.Point) *Edge {
	if e == nil {
		return nil
	}
	if cmp.GeomPointEqual(dest, *e.Dest()) {
		return e
	}
	for ne := e.ONext(); ne != e; ne = ne.ONext() {
		if cmp.GeomPointEqual(dest, *ne.Dest()) {
			return ne
		}
	}
	return nil
}

// DumpAllEdges dumps all the edges as a multiline string
func (e *Edge) DumpAllEdges() string {
	var ml geom.MultiLineString
	ln := e.AsLine()
	ml = append(ml, ln[:])
	cwe := e.OPrev()

	for cwe != e {
		ln := cwe.AsLine()
		ml = append(ml, ln[:])
		cwe = cwe.OPrev()
	}
	return wkt.MustEncode(ml)
}

// IsEqual checks to see if the edges are the same
func (e *Edge) IsEqual(e1 *Edge) bool {
	// first let's get the edge numbers the same
	return e == &e1.qe.e[e.num]
}

// Validate check to se if the edges in the edges are correctly
// oriented
func Validate(e *Edge) error {

	var err ErrInvalid

	el := e.Rot()
	ed := el.Rot()
	er := ed.Rot()

	if ed.Sym() != e {
		// The Sym of Sym should be self
		err = append(err, "invalid Sym")
	}
	if ed != e.Sym() {
		err = append(err, fmt.Sprintf("invalid Rot: left.Rot != e.Sym %p : %p", el, e.Sym()))
	}
	if er != el.Sym() {
		err = append(err, fmt.Sprintf("invalid Rot: rot != e %p : %p", er, el.Sym()))

	}

	if e != el.InvRot() {
		err = append(err, "invalid Rot: rot != esym.InvRot")
	}

	if e.Orig() == nil {
		err = append(err, "expected edge to have origin")
		return err
	}
	if e.Dest() == nil {
		err = append(err, "expected edge to have dest")
		return err
	}

	// Collect edges
	cwe := e.OPrev()
	edges := []*Edge{e}
	pts := make(map[geom.Point]bool)
	pts[*e.Dest()] = true
	pts[*e.Orig()] = true
	for e != cwe {

		if cwe.Orig() == nil {
			err = append(err, "expected edge to have origin")
			return err
		}
		if cwe.Dest() == nil {
			err = append(err, "expected edge to have dest")
			return err
		}

		if !cmp.GeomPointEqual(*e.Orig(), *cwe.Orig()) {
			err = append(err, "orig not equal for edge")
			return err
		}
		if pts[*cwe.Dest()] {
			err = append(err, "dest not unique")
			return err
		}
		pts[*cwe.Dest()] = true

		// check ONext as well
		if edges[len(edges)-1] != cwe.ONext() {
			err = append(err, "expected onext to be inverse of oprev")
			return err
		}

		edges = append(edges, cwe)
		cwe = cwe.OPrev()
	}

	origin := *e.Orig()
	switch len(edges) {
	case 1:
		// there is only one edge.
		if e.Sym() != e.LPrev() {
			err = append(err, "invalid single edge LPrev")
		}
		if e.Sym() != e.RPrev() {
			err = append(err, "invalid single edge RPrev")
		}
		if e.Sym() != e.RNext() {
			err = append(err, fmt.Sprintf("invalid single edge RNext : %p -- %p", e.RNext(), e))
		}
		if e.Sym() != e.LNext() {
			err = append(err, fmt.Sprintf("invalid single edge LNext : %p -- %p", e.LNext(), e))
		}

		if debug && err != nil {
			err = append(err, fmt.Sprintf("edges:\ne  %p\nel %p\ned %p\ner %p\n", e, el, ed, er))
			err = append(err, fmt.Sprintf("edges:\ne  %p\nel %p\ned %p\ner %p\n", e.next, el.next, ed.next, er.next))
			err = append(err, fmt.Sprintf("invalid edge: %v", wkt.MustEncode(e.AsLine())))
		}

	case 2:
		// Nothing left to test

	case 3:
		printErr := false
		// only need to do one test.
		leftDest := *(edges[0].Dest())
		midDest := *(edges[1].Dest())
		rightDest := *(edges[2].Dest())
		lftClass := Classify(origin, midDest, leftDest)
		if lftClass != LEFT && lftClass != RIGHT {
			err = append(err, "3: clockwise line is not left or RIGHT")
			printErr = true
		}
		rtClass := Classify(origin, midDest, rightDest)
		if rtClass != RIGHT && rtClass != LEFT {
			err = append(err,
				fmt.Sprintf(
					"3: counterclockwise line not left or right:%v\n%v %v %v",
					rtClass,
					origin,
					midDest,
					rightDest,
				),
			)
			printErr = true
		}
		if debug && printErr {
			err = append(err, fmt.Sprintf("left invalid edge: \n\t%v\n\t%v\n\t%v",
				wkt.MustEncode(edges[0].AsLine()),
				wkt.MustEncode(edges[1].AsLine()),
				wkt.MustEncode(edges[2].AsLine()),
			))
			err = append(err, edges[0].DumpAllEdges())
		}

	default:
		printErr := false
		for i, j, k := len(edges)-2, len(edges)-1, 0; k < len(edges); i, j, k = j, k, k+1 {
			leftDest := *(edges[k].Dest())
			midDest := *(edges[j].Dest())
			rightDest := *(edges[i].Dest())
			lftClass := Classify(origin, midDest, leftDest)
			if lftClass != LEFT && lftClass != DESTINATION {
				printErr = true
				err = append(err, "clockwise line not left")
			}
			rtClass := Classify(origin, midDest, rightDest)
			if rtClass != RIGHT && rtClass != DESTINATION {
				printErr = true
				err = append(err, "counterclockwise line not right")
			}
			if debug && printErr {
				err = append(err, fmt.Sprintf("left invalid edge: %v:%v:%v",
					wkt.MustEncode(edges[i].AsLine()),
					wkt.MustEncode(edges[j].AsLine()),
					wkt.MustEncode(edges[k].AsLine()),
				))
				err = append(err, edges[i].DumpAllEdges())
			}
		}

	}

	return err
}
