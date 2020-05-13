package quadedge

import (
	"context"
	"fmt"
	"log"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar/intersect"
	"github.com/go-spatial/geom/winding"
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

// OPrev returns the next cw edge around (from) the origin of the current edge.
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

	e.WalkAllONext(func(ee *Edge) bool {
		ln := ee.AsLine()
		ml = append(ml, [][2]float64{ln[0], ln[1]})
		return true
	})
	str, err := wkt.EncodeString(ml)
	if err != nil {
		return err.Error()
	}
	return str
}

func (e *Edge) WalkAllOPrev(fn func(*Edge) (loop bool)) {
	if !fn(e) {
		return
	}
	cwe := e.OPrev()
	for cwe != e {
		if !fn(cwe) {
			return
		}
		cwe = cwe.OPrev()
	}

	// for cwe := e.OPrev(); cwe != e && fn(cwe) ; cwe = e.OPrev(){}

}
func (e *Edge) WalkAllONext(fn func(*Edge) (loop bool)) {
	if !fn(e) {
		return
	}
	ccwe := e.ONext()
	count := 0
	for ccwe != e {
		if !fn(ccwe) {
			return
		}
		ccwe = ccwe.ONext()
		count++
		if count == 100 {
			if debug {
				panic("inifite loop")
			} else {
				log.Printf("infinite loop in WalkAllONext")
				break
			}
		}
	}
}

// IsEqual checks to see if the edges are the same
func (e *Edge) IsEqual(e1 *Edge) bool {
	if e == nil {
		return e1 == nil
	}

	if e1 == nil {
		return e == nil
	}
	// first let's get the edge numbers the same
	return e == &e1.qe.e[e.num]
}

// Validate check to se if the edges in the edges are correctly
// oriented
func Validate(e *Edge, order winding.Order) (err1 error) {

	const radius = 10
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

	if len(err) != 0 {
		return err
	}

	if e.Orig() == nil {
		err = append(err, "expected edge to have origin")
		return err
	}

	orig := *e.Orig()
	seen := make(map[geom.Point]bool)
	points := []geom.Point{}
	e.WalkAllONext(func(ee *Edge) bool {
		dest := ee.Dest()
		if dest == nil {
			err = append(err, "dest is nil")
			return false
		}
		if ee.Orig() == nil {
			err = append(err, "expected edge to have origin")
			return false
		}
		if seen[*dest] {
			err = append(err, "dest not unique")
			err = append(err, ee.DumpAllEdges())
			return false
		}
		seen[*ee.Dest()] = true
		points = append(points, *ee.Dest())

		if !cmp.GeomPointEqual(*ee.Orig(), orig) {
			err = append(
				err,
				fmt.Sprintf(
					"expected edge %v to have same origin %v instead of %v",
					len(points), wkt.MustEncode(orig),
					wkt.MustEncode(*ee.Orig()),
				),
			)
		}
		return true
	})
	if len(err) != 0 {
		return err
	}

	if len(points) > 2 {

		npoints := make([]geom.Point, len(points))
		// First we need to turn all the points into segments
		segs := make([]geom.Line, len(points))
		xprdSum := 0.0

		for j, i := len(points)-1, 0; i < len(points); j, i = i, i+1 {
			segs[i] = geom.Line{points[j], points[i]}
			lpt := [2]float64{
				points[j][0] - orig[0],
				points[j][1] - orig[1],
			}
			pt := [2]float64{
				points[i][0] - orig[0],
				points[i][1] - orig[1],
			}

			npoints[i] = geom.Point(pt)
			xprdSum += sign(xprd(lpt, pt))
		}

		switch sign(xprdSum) {
		case 0:
			// All points are colinear to each other.
			// Need to check winding order with original point
			// not enough information just using the outer points, we need to include the origin
			if !order.OfGeomPoints(append(points, orig)...).IsCounterClockwise() {
				err = append(err,
					fmt.Sprintf("1. expected all points to be counter-clockwise: %v:%v\n%v",
						wkt.MustEncode(orig),
						wkt.MustEncode(points),
						wkt.MustEncode(segs),
					))
			}
		case -1: //counter-clockwise

		case 1: // clockwise
			err = append(err, fmt.Sprintf("2. expected all points to be counter-clockwise: %v:%v\n%v\n%v",
				wkt.MustEncode(orig),
				wkt.MustEncode(points),
				wkt.MustEncode(segs),
				e.DumpAllEdges(),
			))

		}

		// New we need to check that there are no self intersecting lines.
		eq := intersect.NewEventQueue(segs)
		eq.CMP = cmp
		_ = eq.FindIntersects(
			context.Background(),
			true,
			func(src, dest int, pt [2]float64) error {
				// make sure the point is not an end point
				gpt := geom.Point(pt)
				if (cmp.GeomPointEqual(gpt, *segs[src].Point1()) || cmp.GeomPointEqual(gpt, *segs[src].Point2())) &&
					(cmp.GeomPointEqual(gpt, *segs[dest].Point1()) || cmp.GeomPointEqual(gpt, *segs[dest].Point2())) {
					return nil
				}
				// the second point in each segment should be the vertex we care about.
				// this is because of the way we build up the segments above.
				err = append(err,
					fmt.Sprintf("found self interstion for vertices %v and %v",
						wkt.MustEncode(*segs[src].Point2()),
						wkt.MustEncode(*segs[dest].Point2()),
					),
				)
				return err
			},
		)
	}

	if len(err) == 0 {
		return nil
	}
	return err
}
