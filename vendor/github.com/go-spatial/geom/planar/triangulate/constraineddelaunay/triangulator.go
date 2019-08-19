package constraineddelaunay

import (
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar/triangulate"
	"github.com/go-spatial/geom/planar/triangulate/quadedge"
)

var ErrLinesDoNotIntersect = errors.New("line segments do not intersect")
var ErrMismatchedLengths = errors.New("the arguments lengths do not match")

// these errors indicate a problem with the algorithm.
var ErrUnableToUpdateVertexIndex = errors.New("unable to update vertex index")
var ErrUnexpectedDeadNode = errors.New("unexpected dead node")
var ErrCoincidentEdges = errors.New("coincident edges")

/*
Triangulator provides methods for performing a constrained delaunay
triangulation.

Domiter, Vid. "Constrained Delaunay triangulation using plane subdivision."
Proceedings of the 8th central European seminar on computer graphics.
Budmerice. 2004.
http://old.cescg.org/CESCG-2004/web/Domiter-Vid/CDT.pdf
*/
type Triangulator struct {
	builder *triangulate.DelaunayTriangulationBuilder
	// a map of constraints where the segments have the lesser point first.
	constraints map[triangulate.Segment]bool
	subdiv      *quadedge.QuadEdgeSubdivision
	tolerance   float64
	// run validation after many modification operations. This is expensive,
	// but very useful when debugging.
	validate bool
	// maintain an index of vertices to quad edges. Each vertex will point to
	// one quad edge that has the vertex as an origin. The other quad edges
	// that point to this vertex can be reached from there.
	vertexIndex map[quadedge.Vertex]*quadedge.QuadEdge
}

/*
addDataToEdge appends this data to the data associated with an edge. If the
edge doesn't already contain an array, the array is created before the data is
appended.
*/
func addDataToEdge(qe *quadedge.QuadEdge, data interface{}) {
	if data != nil {
		if qe.GetData() == nil {
			qe.SetData(make([]interface{}, 0))
		}
		arr, ok := qe.GetData().([]interface{})
		if !ok {
			log.Fatalf("could not cast data to array of interfaces.")
		}
		arr = append(arr, data)
		qe.SetData(arr)
		qe.Sym().SetData(arr)
	}
}

/*
appendNonRepeat only appends the provided value if it does not repeat the last
value that was appended onto the array.
*/
func appendNonRepeat(arr []quadedge.Vertex, v quadedge.Vertex) []quadedge.Vertex {
	if len(arr) == 0 || arr[len(arr)-1].Equals(v) == false {
		arr = append(arr, v)
	}
	return arr
}

/*
createSegment creates a segment with vertices a & b, if it doesn't already
exist. All the vertices must already exist in the triangulator.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) createSegment(s triangulate.Segment, data interface{}) error {
	if s.GetStart().Equals(s.GetEnd()) {
		return fmt.Errorf("segment must not have the same start/end (%v/%v)", s.GetStart(), s.GetEnd())
	}

	qe, err := tri.LocateSegment(s.GetStart(), s.GetEnd())

	if _, ok := err.(quadedge.ErrLocateFailure); err != nil && !ok {
		return err
	}
	if qe != nil {
		// if the segment already exists
		addDataToEdge(qe, data)
		return nil
	}

	ct, err := tri.findIntersectingTriangle(s)

	if err != nil && err != ErrCoincidentEdges {
		return err
	}
	from := ct.Qe.Sym()

	ct, err = tri.findIntersectingTriangle(triangulate.NewSegment(geom.Line{s.GetEnd(), s.GetStart()}))
	if err != nil && err != ErrCoincidentEdges {
		return err
	}
	to := ct.Qe.OPrev()

	qe = quadedge.Connect(from, to)
	addDataToEdge(qe, data)

	// since we aren't adding any vertices we don't need to modify the vertex
	// index.
	return nil
}

/*
createTriangle creates a triangle with vertices a, b and c. All the vertices
must already exist in the triangulator. Any existing edges that make up the triangle will not be recreated.

This method makes no effort to ensure the resulting changes are a valid
triangulation.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) createTriangle(a, b, c quadedge.Vertex) error {
	if debug {
		log.Printf("createTriangle")
	}
	if err := tri.createSegment(triangulate.NewSegment(geom.Line{a, b}), nil); err != nil {
		return err
	}

	if err := tri.createSegment(triangulate.NewSegment(geom.Line{b, c}), nil); err != nil {
		return err
	}

	if err := tri.createSegment(triangulate.NewSegment(geom.Line{c, a}), nil); err != nil {
		return err
	}

	return nil
}

/*
deleteEdge deletes the specified edge and updates all associated neighbors to
reflect the removal. The local vertex index is also updated to reflect the
deletion.

It is invalid to call this method on the last edge that links to a vertex.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) deleteEdge(e *quadedge.QuadEdge) error {

	toRemove := make(map[*quadedge.QuadEdge]bool, 4)

	eSym := e.Sym()
	eRot := e.Rot()
	eRotSym := e.Rot().Sym()

	// a set of all the edges that will be removed.
	toRemove[e] = true
	toRemove[eSym] = true
	toRemove[eRot] = true
	toRemove[eRotSym] = true

	// remove this edge from the vertex index.
	if err := tri.removeEdgesFromVertexIndex(toRemove, e.Orig()); err != nil {
		return err
	}
	if err := tri.removeEdgesFromVertexIndex(toRemove, e.Dest()); err != nil {
		return err
	}
	quadedge.Splice(e.OPrev(), e)
	quadedge.Splice(eSym.OPrev(), eSym)

	// TODO: this call is horribly inefficient and should be optimized.
	tri.subdiv.Delete(e)

	return nil
}

/*
findIntersectingTriangle finds the triangle that shares the vertex s.GetStart()
and intersects at least part of the edge that extends from s.GetStart().

Tolerance is not considered when determining if vertices are the same.

Returns a quadedge that has s.GetStart() as the origin and the right face is
the desired triangle. If the segment falls on an edge, the triangle to the
right of the segment is returned.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) findCoincidentEdge(s triangulate.Segment) (*quadedge.QuadEdge, error) {

	start, err := tri.locateEdgeByVertex(s.GetStart())
	if err != nil {
		return nil, err
	}

	qe := start

	// walk around all the edges that share qe.Orig()
	for {
		qe = qe.OPrev()

		if qe.IsLive() == false {
			return nil, ErrUnexpectedDeadNode
		}

		lc := s.GetEnd().Classify(qe.Orig(), qe.Dest())

		if lc == quadedge.BETWEEN || lc == quadedge.DESTINATION || lc == quadedge.BEYOND {
			// if s is between the two edges, we found edge
			return qe, nil
		}

		if qe == start {
			// if we've walked all the way around the vertex.
			break
		}
	}

	return nil, fmt.Errorf("no coincident edge: %v", s)
}

/*
findIntersectingTriangle finds the triangle that shares the vertex s.GetStart()
and intersects at least part of the edge that extends from s.GetStart().

Tolerance is not considered when determining if vertices are the same.

Returns a quadedge that has s.GetStart() as the origin and the right face is
the desired triangle. If the segment falls on an edge, the triangle to the
right of the segment is returned.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) findIntersectingTriangle(s triangulate.Segment) (*Triangle, error) {

	qe, err := tri.locateEdgeByVertex(s.GetStart())
	if err != nil {
		return nil, err
	}

	left := qe

	// walk around all the triangles that share qe.Orig()
	for {
		if left.IsLive() == false {
			return nil, ErrUnexpectedDeadNode
		}
		// create the two quad edges around s
		right := left.OPrev()

		lc := s.GetEnd().Classify(left.Orig(), left.Dest())
		rc := s.GetEnd().Classify(right.Orig(), right.Dest())

		if (lc == quadedge.RIGHT && rc == quadedge.LEFT) || lc == quadedge.BETWEEN || lc == quadedge.DESTINATION || lc == quadedge.BEYOND {
			// if s is between the two edges, we found our triangle.
			return &Triangle{left}, nil
		} else if lc != quadedge.RIGHT && lc != quadedge.LEFT && rc != quadedge.LEFT && rc != quadedge.RIGHT {
			return &Triangle{left}, ErrCoincidentEdges
		}
		// } else if lc != quadedge.RIGHT && lc != quadedge.LEFT {
		// 	return &Triangle{left}, nil
		// } else if rc != quadedge.LEFT && rc != quadedge.RIGHT {
		// 	return &Triangle{right}, nil
		// }
		left = right

		if left == qe {
			// if we've walked all the way around the vertex.
			return nil, fmt.Errorf("no intersecting triangle: %v", s)
		}
	}

	return nil, fmt.Errorf("no intersecting triangle: %v", s)
}

/*
GetEdges gets the edges of the computed triangulation as a MultiLineString.

returns the edges of the triangulation

If tri is nil a panic will occur.
*/
func (tri *Triangulator) GetEdges() geom.MultiLineString {
	return tri.builder.GetEdges()
}

/*
Returns a triangle that is outside the geometry.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) GetExteriorTriangle() *Triangle {
	result := &Triangle{tri.subdiv.GetEdges()[0].Sym()}
	return result.Normalize()
}

/*
Returns the subdivision used by this triangulator.

This is provided for read-only access. Making changes will result in undefined
behaviour.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) GetSubdivision() *quadedge.QuadEdgeSubdivision {
	return tri.subdiv
}

/*
GetTriangles Gets the faces of the computed triangulation as a
MultiPolygon.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) GetTriangles() (geom.MultiPolygon, error) {
	return tri.builder.GetTriangles()
}

/*
InsertGeometry is a convenience function that wraps InsertGeometries.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) InsertGeometry(g geom.Geometry) error {
	return tri.InsertGeometries([]geom.Geometry{g}, nil)
}

/*
InsertGeometries inserts the line segments in the specified geometries and
builds a triangulation. The line segments are used as constraints in the
triangulation. If the geometry is made up solely of points, then no
constraints will be used.

g contains a list of the geometries to insert.

data contains a list of the data values that should be associated with the
constraints in each geometry. This should either be empty, or the same number
of arguments as g.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) InsertGeometries(g []geom.Geometry, data []interface{}) error {
	if len(data) != 0 && len(g) != len(data) {
		return ErrMismatchedLengths
	}

	if err := tri.insertSites(g...); err != nil {
		if debug {
			log.Printf("got error for insertSites: %v", err)
		}
		return err
	}

	tri.constraints = make(map[triangulate.Segment]bool)

	for i, gm := range g {
		var d interface{}
		if len(data) > 0 {
			d = data[i]
		}

		if err := tri.insertConstraints(gm, d); err != nil {
			if debug {
				log.Printf("got error for insertConstraints: %v", err)
			}
			return err
		}
	}

	return nil
}

/*
insertSites inserts all of the vertices found in g into a Delaunay
triangulation. Other steps will modify the Delaunay Triangulation to create
the constrained Delaunay triangulation.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) insertSites(g ...geom.Geometry) error {

	tri.builder = triangulate.NewDelaunayTriangulationBuilder(tri.tolerance)

	err := tri.builder.SetSites(g...)
	if err != nil {
		return err
	}
	tri.subdiv = tri.builder.GetSubdivision()

	// Add all the edges to a constant time lookup
	tri.vertexIndex = make(map[quadedge.Vertex]*quadedge.QuadEdge)
	edges := tri.subdiv.GetEdges()
	for i := range edges {
		e := edges[i]
		if _, ok := tri.vertexIndex[e.Orig()]; ok == false {
			tri.vertexIndex[e.Orig()] = e
		}
		if _, ok := tri.vertexIndex[e.Dest()]; ok == false {
			tri.vertexIndex[e.Dest()] = e.Sym()
		}
	}

	return nil
}

/*
insertConstraints modifies the triangulation by incrementally using the
line segements in g as constraints in the triangulation. After this step
the triangulation is no longer a proper Delaunay triangulation, but the
constraints are guaranteed. Some constraints may need to be split (think
about the case when two constraints intersect).

If tri is nil a panic will occur.
*/
func (tri *Triangulator) insertConstraints(g geom.Geometry, data interface{}) error {
	lines, err := geom.ExtractLines(g)
	if err != nil {
		return fmt.Errorf("error adding constraint: %v", err)
	}
	constraints := make(map[triangulate.Segment]bool)
	for _, l := range lines {
		// make the line ordering consistent
		if !cmp.PointLess(l[0], l[1]) {
			l[0], l[1] = l[1], l[0]
		}

		seg := triangulate.NewSegment(l)
		// this maintains the constraints and de-dupes
		constraints[seg] = true
		tri.constraints[seg] = true
	}

	for seg := range constraints {
		tmp := seg.DeepCopy()
		// find locations where the constrained edges intersect and insert new // sites at the intersections.
		if err := tri.insertIntersectionSites(&tmp); err != nil {
			return fmt.Errorf("error adding constraint: %v", err)
		}
		if debug {
			log.Printf("=== insertIntersectionSites complete %v", tri.subdiv.DebugDumpEdges())
		}
		if err = tri.Validate(); err != nil {
			if debug {
				log.Printf("=== failed validation %v", err)
			}
			return err
		}
		tmp = seg.DeepCopy()
		if err := tri.insertEdgeCDT(&tmp, data); err != nil {
			return fmt.Errorf("error adding constraint: %v", err)
		}
		if debug {
			log.Printf("=== Insert constraint complete %v", tri.subdiv.DebugDumpEdges())
		}
		if err = tri.Validate(); err != nil {
			return err
		}
	}

	return nil
}

/*
intersection calculates the intersection between two line segments. When the
rest of geom is ported over from spatial, this can be replaced with a more
generic call.

The tolerance here only acts by extending the lines by tolerance. E.g. if the
tolerance is 0.1 and you have two lines {{0, 0}, {1, 0}} and
{{0, 0.01}, {1, 0.01}} then these will not be marked as intersecting lines.

If tolerance is used to mark two lines as intersecting, you are still
guaranteed that the intersecting point will fall _on_ one of the lines, not in
the extended region of the line.

Taken from: https://stackoverflow.com/questions/563198/how-do-you-detect-where-two-line-segments-intersect

If tri is nil a panic will occur.
*/
func (tri *Triangulator) intersection(l1, l2 triangulate.Segment) (quadedge.Vertex, error) {
	p := l1.GetStart()
	r := l1.GetEnd().Sub(p)
	q := l2.GetStart()
	s := l2.GetEnd().Sub(q)

	rs := r.CrossProduct(s)

	if rs == 0 {
		return quadedge.Vertex{}, ErrLinesDoNotIntersect
	}
	t := q.Sub(p).CrossProduct(s.Divide(r.CrossProduct(s)))
	u := p.Sub(q).CrossProduct(r.Divide(s.CrossProduct(r)))

	// calculate the acceptable range of values for t
	ttolerance := tri.tolerance / r.Magn()
	tlow := -ttolerance
	thigh := 1 + ttolerance

	// calculate the acceptable range of values for u
	utolerance := tri.tolerance / s.Magn()
	ulow := -utolerance
	uhigh := 1 + utolerance

	if t < tlow || t > thigh || u < ulow || u > uhigh {
		return quadedge.Vertex{}, ErrLinesDoNotIntersect
	}
	// if t is just out of range, but within the acceptable tolerance, snap
	// it back to the beginning/end of the line.
	t = math.Min(1, math.Max(t, 0))

	return p.Sum(r.Times(t)), nil
}

/*
IsConstraint returns true if e is a constrained edge.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) IsConstraint(e *quadedge.QuadEdge) bool {

	_, ok := tri.constraints[triangulate.NewSegment(geom.Line{e.Orig(), e.Dest()})]
	if ok {
		return true
	}
	_, ok = tri.constraints[triangulate.NewSegment(geom.Line{e.Dest(), e.Orig()})]
	return ok
}

/*
insertCoincidentEdge inserts an edge that shares a vertex a with another edge
that is coincident with ab.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) insertCoincidentEdge(ab *triangulate.Segment, data interface{}) error {
	if debug {
		log.Printf("insertCoincidentEdge %v", ab)
	}

	// find the coincident edge
	ce, err := tri.findCoincidentEdge(*ab)
	if err != nil {
		return err
	}

	c := ce.Dest().Classify(ab.GetStart(), ab.GetEnd())

	switch {
	// ab should always be longer than c
	case c == quadedge.BETWEEN:
		addDataToEdge(ce, data)
		vb := triangulate.NewSegment(geom.Line{ce.Dest(), ab.GetEnd()})
		tri.insertEdgeCDT(&vb, data)

	default:
		return fmt.Errorf("invalid point classification: %v", c)
	}

	return nil
}

/*
insertCoincidentEdge inserts an edge that shares a vertex a with another edge
that is coincident with ab.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) insertCoincidentEdgeSites(ab *triangulate.Segment) error {

	// find the coincident edge
	ce, err := tri.findCoincidentEdge(*ab)
	if err != nil {
		return err
	}

	c := ce.Dest().Classify(ab.GetStart(), ab.GetEnd())

	switch {
	case c == quadedge.BEYOND:
		// split the coincident edge where ab ends
		if err := tri.splitEdge(ce, ab.GetEnd()); err != nil {
			return err
		}

	case c == quadedge.BETWEEN:
		// continue inserting sites where ce ends
		vb := triangulate.NewSegment(geom.Line{ce.Dest(), ab.GetEnd()})
		tri.insertIntersectionSites(&vb)

	default:
		return fmt.Errorf("invalid point classification: %v", c)
	}

	return nil
}

/*
insertEdgeCDT attempts to follow the pseudo code in Domiter.

Procedure InsertEdgeCDT(T:CDT, ab:Edge)

There are some deviations that are also mentioned inline in the comments

 - Some aparrent typos that are resolved to give consistent results
 - Modifications to work with the planar subdivision representation of
   a triangulation (QuadEdge)
 - Modification to support the case when two constrained edges intersect
   at more than the end points.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) insertEdgeCDT(ab *triangulate.Segment, data interface{}) error {

	if debug {
		log.Printf("ab: %v tri: %v", ab, tri.subdiv.DebugDumpEdges())
	}
	qe, err := tri.LocateSegment(ab.GetStart(), ab.GetEnd())
	if qe != nil && err != nil {
		return fmt.Errorf("error inserting constraint: %v", err)
	}
	if qe != nil {
		// nothing to change, the edge already exists. Just append the data
		addDataToEdge(qe, data)
		return nil
	}

	// Precondition: a,b in T and ab not in T
	// Find the triangle t ∈ T that contains a and is cut by ab
	t, err := tri.findIntersectingTriangle(*ab)
	if err == ErrCoincidentEdges {
		return tri.insertCoincidentEdge(ab, data)
	} else if err != nil {
		return err
	}
	if debug {
		log.Printf("ab: %v t: %v", ab, t)
	}

	removalList := make([]*quadedge.QuadEdge, 0)

	// PU:=EmptyList
	pu := make([]quadedge.Vertex, 0)
	// PL:=EmptyList
	pl := make([]quadedge.Vertex, 0)
	// v:=a
	v := ab.GetStart()
	b := ab.GetEnd()

	// While v not in t do -- should this be 'b not in t'!? -JRS
	for t.IntersectsPoint(b) == false {
		if debug {
			log.Printf("t: %v v: %v", t, v)
		}
		// tseq:=OpposedTriangle(t,v)
		tseq, err := t.opposedTriangle(v)
		if err != nil {
			return err
		}
		// vseq:=OpposesdVertex(tseq,t)
		vseq, err := tseq.opposedVertex(t)
		if err != nil {
			return err
		}

		shared, err := t.sharedEdge(tseq)
		if err != nil {
			return err
		}

		c := vseq.Classify(ab.GetStart(), ab.GetEnd())
		if debug {
			log.Printf("t: %v tseq: %v", t, tseq)
		}

		// should we remove the edge shared between t & tseq?
		flagEdgeForRemoval := false

		abOnOrig := tri.subdiv.IsOnLine(ab.GetLineSegment(), shared.Orig())
		abOnDest := tri.subdiv.IsOnLine(ab.GetLineSegment(), shared.Dest())

		switch {

		case abOnOrig:
			// InsertEdgeCDT(T, vseqb)
			vb := triangulate.NewSegment(geom.Line{shared.Orig(), ab.GetEnd()})
			if err := tri.insertEdgeCDT(&vb, data); err != nil {
				return err
			}
			// a:=vseq -- Should this be b:=vseq!? -JRS
			b = shared.Orig()
			*ab = triangulate.NewSegment(geom.Line{ab.GetStart(), b})

		case abOnDest:
			// InsertEdgeCDT(T, vseqb)
			vb := triangulate.NewSegment(geom.Line{shared.Dest(), ab.GetEnd()})
			if err := tri.insertEdgeCDT(&vb, data); err != nil {
				return err
			}
			// a:=vseq -- Should this be b:=vseq!? -JRS
			b = shared.Dest()
			*ab = triangulate.NewSegment(geom.Line{ab.GetStart(), b})

		case c == quadedge.BETWEEN:
			if debug {
				log.Printf("vseq: %v", vseq)
				log.Printf("shared: %v", shared)
				log.Printf("ab: %v", ab)
				log.Printf("subdiv: %v", tri.subdiv.DebugDumpEdges())
				log.Printf("tolerance: %v", tri.tolerance)
			}

			// if tri.subdiv.IsOnLine(ab.GetLineSegment(), shared.Orig()) {
			// 	b = shared.Orig()
			// } else if tri.subdiv.IsOnLine(ab.GetLineSegment(), shared.Dest()) {
			// 	b = shared.Dest()
			// }

			// InsertEdgeCDT(T, vseqb)
			vb := triangulate.NewSegment(geom.Line{vseq, ab.GetEnd()})
			if err := tri.insertEdgeCDT(&vb, data); err != nil {
				return err
			}

			b = vseq
			*ab = triangulate.NewSegment(geom.Line{ab.GetStart(), b})
			if debug {
				log.Printf("new ab: %v", *ab)
			}
			flagEdgeForRemoval = true

		// if the constrained edge is passing through another constrained edge
		case tri.IsConstraint(shared):
			// find the point of intersection
			iv, err := tri.intersection(*ab, triangulate.NewSegment(geom.Line{shared.Orig(), shared.Dest()}))
			if debug {
				log.Printf("iv: %v ab: %v shared: %v", iv, ab, shared)
			}
			if err != nil {
				return err
			}

			// split the constrained edge we interesect
			if err := tri.splitEdge(shared, iv); err != nil {
				return err
			}
			tri.deleteEdge(shared)
			tseq, err = t.opposedTriangle(v)
			if err != nil {
				return err
			}

			// create a new edge for the rest of this segment and recursively
			// insert the new edge.
			vb := triangulate.NewSegment(geom.Line{iv, ab.GetEnd()})
			tri.insertEdgeCDT(&vb, data)

			// the current insertion will stop at the interesction point
			b = iv
			*ab = triangulate.NewSegment(geom.Line{ab.GetStart(), iv})

		// If vseq above the edge ab then
		case c == quadedge.LEFT:
			// v:=Vertex shared by t and tseq above ab
			v = shared.Orig()
			pu = appendNonRepeat(pu, v)
			// AddList(PU ,vseq)
			pu = appendNonRepeat(pu, vseq)
			flagEdgeForRemoval = true

		// Else If vseq below the edge ab
		case c == quadedge.RIGHT:
			// v:=Vertex shared by t and tseq below ab
			v = shared.Dest()
			pl = appendNonRepeat(pl, v)
			// AddList(PL, vseq)
			pl = appendNonRepeat(pl, vseq)
			flagEdgeForRemoval = true

		case c == quadedge.DESTINATION:
			flagEdgeForRemoval = true

		default:
			return fmt.Errorf("invalid point classification: %v", c)
		}

		if flagEdgeForRemoval {
			// "Remove t from T" -- We are just removing the edge intersected
			// by ab, which in effect removes the triangle.
			removalList = append(removalList, shared)
		}

		t = tseq
	}
	// EndWhile

	if ab.GetStart().Equals(ab.GetEnd()) == false {
		// remove the previously marked edges
		for i := range removalList {
			tri.deleteEdge(removalList[i])
		}

		// TriangulatePseudoPolygon(PU,ab,T)
		if err := tri.triangulatePseudoPolygon(pu, *ab); err != nil {
			return err
		}
		// TriangulatePseudoPolygon(PL,ab,T)
		if err := tri.triangulatePseudoPolygon(pl, *ab); err != nil {
			return err
		}

		// Add edge ab to T
		if err := tri.createSegment(*ab, data); err != nil {
			return err
		}
		tri.constraints[*ab] = true
	}

	return nil
}

/*
If tri is nil a panic will occur.
*/
func (tri *Triangulator) insertIntersectionSites(ab *triangulate.Segment) error {

	if debug {
		log.Printf("ab: %v tri: %v", ab, tri.subdiv.DebugDumpEdges())
	}
	qe, err := tri.LocateSegment(ab.GetStart(), ab.GetEnd())
	if qe != nil && err != nil {
		return fmt.Errorf("error inserting constraint: %v", err)
	}
	if qe != nil {
		return nil
	}

	// Precondition: a,b in T and ab not in T
	// Find the triangle t ∈ T that contains a and is cut by ab
	t, err := tri.findIntersectingTriangle(*ab)
	if err == ErrCoincidentEdges {
		return tri.insertCoincidentEdgeSites(ab)
	} else if err != nil {
		return err
	}

	// v:=a
	v := ab.GetStart()
	b := ab.GetEnd()

	// While b not in t do
	for t.IntersectsPoint(b) == false {
		// tseq:=OpposedTriangle(t,v)
		tseq, err := t.opposedTriangle(v)
		if err != nil {
			return err
		}
		// vseq:=OpposesdVertex(tseq,t)
		vseq, err := tseq.opposedVertex(t)
		if err != nil {
			return err
		}

		shared, err := t.sharedEdge(tseq)
		if err != nil {
			return err
		}
		if debug {

			log.Printf("t: %v tseq: %v", t, tseq)

			log.Printf("constraint: %v shared: %v", tri.IsConstraint(shared), shared)
		}

		// if the constrained edge is passing through another constrained edge
		if tri.IsConstraint(shared) {
			// find the point of intersection
			iv, err := tri.intersection(*ab, triangulate.NewSegment(geom.Line{shared.Orig(), shared.Dest()}))
			if debug {
				log.Printf("iv: %v ab: %v shared: %v", iv, ab, shared)
			}
			if err != nil {
				return err
			}

			// split the constrained edge we interesect
			if err := tri.splitEdge(shared, iv); err != nil {
				return err
			}
			tseq, err = t.opposedTriangle(v)
			if err != nil {
				return err
			}

			// Recursively start a new interseciton from iv to the end.
			biv := triangulate.NewSegment(geom.Line{iv, ab.GetEnd()})
			if err := tri.insertIntersectionSites(&biv); err != nil {
				return err
			}

			// the current insertion will stop at the interesction point
			b = iv
			*ab = triangulate.NewSegment(geom.Line{ab.GetStart(), b})

		} else {
			c := vseq.Classify(ab.GetStart(), ab.GetEnd())
			if debug {
				log.Printf("t: %v tseq: %v", t, tseq)
			}

			switch {
			case tri.subdiv.IsOnLine(ab.GetLineSegment(), shared.Orig()):
				// TODO should this check to see if we extend past the overlap?
				return nil

			case tri.subdiv.IsOnLine(ab.GetLineSegment(), shared.Dest()):
				// TODO should this check to see if we extend past the overlap?
				return nil

			// If vseq above the edge ab then
			case c == quadedge.LEFT:
				// v:=Vertex shared by t and tseq above ab
				v = shared.Orig()

			// Else If vseq below the edge ab
			case c == quadedge.RIGHT:
				// v:=Vertex shared by t and tseq below ab
				v = shared.Dest()

			case c == quadedge.BETWEEN:
				v = vseq

			case c == quadedge.DESTINATION:
				// no-op, all done

			default:
				return fmt.Errorf("invalid point classification: %v", c)
			}
		}

		t = tseq
	}
	// EndWhile

	return nil
}

/*
isInCircle is a method that ensures the points are CCW before checking for circle containment
*/
func isInCircle(a, b, c, p quadedge.Vertex) bool {
	if a.IsCCW(b, c) == false {
		return p.IsInCircle(b, a, c)
	}
	return p.IsInCircle(a, b, c)
}

/*
locateEdgeByVertex finds a quad edge that has this vertex as Orig(). This will
not be a unique edge.

This is looking for an exact match and tolerance will not be considered.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) locateEdgeByVertex(v quadedge.Vertex) (*quadedge.QuadEdge, error) {
	qe := tri.vertexIndex[v]

	if qe == nil {
		return nil, quadedge.ErrLocateFailure{V: &v}
	}
	return qe, nil
}

/*
locateEdgeByVertex finds a quad edge that has this vertex as Orig(). This will
not be a unique edge.

This is looking for an exact match and tolerance will not be considered.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) LocateSegment(v1 quadedge.Vertex, v2 quadedge.Vertex) (*quadedge.QuadEdge, error) {
	qe := tri.vertexIndex[v1]

	if qe == nil {
		return nil, quadedge.ErrLocateFailure{V: &v1}
	}

	start := qe
	for {
		if qe == nil || qe.IsLive() == false {
			if debug {
				log.Printf("unexpected dead node: %v", qe)
			}
			return nil, fmt.Errorf("nil or dead qe when locating segment %v %v", v1, v2)
		}
		if v2.Equals(qe.Dest()) {
			return qe, nil
		}

		qe = qe.ONext()
		if qe == start {
			return nil, quadedge.ErrLocateFailure{V: &v2}
		}
	}

	return qe, nil
}

/*
removeConstraintEdge removes any constraints that share the same Orig() and
Dest() as the edge provided. If there are none, no changes are made.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) removeConstraintEdge(e *quadedge.QuadEdge) {
	delete(tri.constraints, triangulate.NewSegment(geom.Line{e.Orig(), e.Dest()}))
	delete(tri.constraints, triangulate.NewSegment(geom.Line{e.Dest(), e.Orig()}))
}

/*
removeEdgesFromVertexIndex will remove a set of QuadEdges from the vertex index
for the specified vertex. If the operation cannot be completed an error will be
returned and the index will not be modified.

The vertex index maps from a vertex to an arbitrary QuadEdges. This method is
helpful in modifying the index after an edge has been deleted.

toRemove - a set of QuadEdges that should be removed from the index. These
QuadEdges don't necessarily have to link to the provided vertex.
v - The vertex to modify in the index.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) removeEdgesFromVertexIndex(toRemove map[*quadedge.QuadEdge]bool, v quadedge.Vertex) error {
	ve := tri.vertexIndex[v]
	if toRemove[ve] {
		for testEdge := ve.ONext(); ; testEdge = testEdge.ONext() {
			if testEdge == ve {
				// if we made it all the way around the vertex without finding
				// a valid edge to reference from this vertex
				return ErrUnableToUpdateVertexIndex
			}
			if toRemove[testEdge] == false {
				tri.vertexIndex[v] = testEdge
				return nil
			}
		}
	}
	// this should happen if the vertex doesn't need to be updated.
	return nil
}

/*
splitEdge splits the given edge at the vertex v. When this happens it creates
two quadrilaterals. Rather than attempt to maintain the Delaunay properties,
this will simply add two more edges from the vertex v to maintain the
triangulation.

While we may lose our Delaunay properties here, this isn't such a big deal as
the constraints can also nullify the Delaunay properties.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) splitEdge(e *quadedge.QuadEdge, v quadedge.Vertex) error {
	if e.Orig().Equals(v) || e.Dest().Equals(v) {
		return nil
	}

	constraint := tri.IsConstraint(e)
	if debug {
		log.Printf("splitEdge e: %v v: %v", e, v)
	}

	ePrev := e.OPrev()
	eSym := e.Sym()
	eSymPrev := eSym.OPrev()

	tri.removeConstraintEdge(e)

	e1 := tri.subdiv.MakeEdge(e.Orig(), v)
	e2 := tri.subdiv.MakeEdge(e.Dest(), v)

	if _, ok := tri.vertexIndex[v]; ok == false {
		tri.vertexIndex[v] = e1.Sym()
	}

	// splice e1 on
	quadedge.Splice(ePrev, e1)
	// splice e2 on
	quadedge.Splice(eSymPrev, e2)

	// splice e1 and e2 together
	quadedge.Splice(e1.Sym(), e2.Sym())

	if err := tri.deleteEdge(e); err != nil {
		return err
	}

	if constraint {
		tri.constraints[triangulate.NewSegment(geom.Line{e1.Orig(), e1.Dest()})] = true
		tri.constraints[triangulate.NewSegment(geom.Line{e2.Dest(), e2.Orig()})] = true
	}

	if e.GetData() != nil {
		e1.SetData(e.GetData())

		arr, ok := e.GetData().([]interface{})
		// this should never happen
		if !ok {
			log.Fatalf("could not cast data to array of interfaces.")
		}
		newArr := make([]interface{}, len(arr))
		copy(newArr, arr)
		e2.SetData(newArr)

		// keep these linked so changing one changes the sym.
		e1.Sym().SetData(e1.GetData())
		e2.Sym().SetData(e2.GetData())
	}

	if debug {
		log.Printf("e: %v subdiv: %v", e, tri.subdiv.DebugDumpEdges())
	}
	t1 := Triangle{e1}
	t2 := Triangle{e2}
	if debug {
		log.Printf("e1: %v t1: %v", e1, &t1)
		log.Printf("e2: %v t2: %v", e2, &t2)
	}
	if t1.IsValid() == false {
		if debug {
			log.Printf("adding: %v", geom.Line{v, e1.RNext().Orig()})
		}
		if err := tri.createSegment(triangulate.NewSegment(geom.Line{v, e1.RNext().Orig()}), nil); err != nil {
			return err
		}
		if t1.IsValid() == false {
			return fmt.Errorf("t1 is still invalid: %v", &t1)
		}
	}
	if t2.IsValid() == false {
		if debug {
			log.Printf("adding: %v", geom.Line{v, e2.RNext().Orig()})
		}
		if err := tri.createSegment(triangulate.NewSegment(geom.Line{v, e2.RNext().Orig()}), nil); err != nil {
			return err
		}
		if t2.IsValid() == false {
			return fmt.Errorf("t2 is still invalid: %v", &t2)
		}
	}

	// since we aren't adding any vertices we don't need to modify the vertex
	// index.
	return nil
}

/*
triangulatePseudoPolygon is taken from the pseudocode TriangulatePseudoPolygon
from Figure 10 in Domiter.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) triangulatePseudoPolygon(p []quadedge.Vertex, ab triangulate.Segment) error {
	if debug {
		log.Printf("triangulatePseudoPolygon(%v, %v)", p, ab)
	}

	// triangulatePseudoPolygon([[0.2 0.3] [0 0]], {[[0 1] [1 0.5]] <nil>})
	a := ab.GetStart()
	b := ab.GetEnd()
	var c quadedge.Vertex
	// If P has more than one element then
	if len(p) > 1 {
		// c:=First vertex of P
		c = p[0]
		ci := 0
		// For each vertex v in P do
		for i, v := range p {
			// If v ∈ CircumCircle (a, b, c) then
			if isInCircle(a, b, c, v) {
				c = v
				ci = i
			}
		}
		// Divide P into PE and PD giving P=PE+c+PD
		pe := p[0:ci]
		pd := p[ci+1:]
		// TriangulatePseudoPolygon(PE, ac, T)
		if err := tri.triangulatePseudoPolygon(pe, triangulate.NewSegment(geom.Line{a, c})); err != nil {
			return err
		}
		// TriangulatePseudoPolygon(PD, cd, T) (cb instead of cd? -JRS)
		if err := tri.triangulatePseudoPolygon(pd, triangulate.NewSegment(geom.Line{c, b})); err != nil {
			return err
		}
	} else if len(p) == 1 {
		c = p[0]
	}

	// If P is not empty then
	if len(p) > 0 && a.Equals(c) == false && b.Equals(c) == false {
		// Add triangle with vertices a, b, c into T
		if err := tri.createTriangle(a, c, b); err != nil {
			return err
		}
	}

	return nil
}

/*
validate runs a number of self consistency checks against a triangulation and
reports the first error.

This is most useful when testing/debugging.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) Validate() error {
	if tri.validate == false {
		return nil
	}

	if err := tri.subdiv.Validate(); err != nil {
		if debug {
			log.Printf("subdiv is invalid: %v", err)
		}
		return err
	}
	if err := tri.validateTriangles(); err != nil {
		if debug {
			log.Printf("validateTriangles is invalid: %v", err)
		}
		return err
	}
	if err := tri.validateVertexIndex(); err != nil {
		if debug {
			log.Printf("validateVertexIndex is invalid: %v", err)
		}
		return err
	}
	return nil
}

/*
validateVertexIndex self consistency checks against a triangulation and the
subdiv and reports the first error.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) validateVertexIndex() error {
	// collect a set of all edges
	edgeSet := make(map[*quadedge.QuadEdge]bool)
	vertexSet := make(map[quadedge.Vertex]bool)
	edges := tri.subdiv.GetPrimaryEdges(true)
	for i := range edges {
		edgeSet[edges[i]] = true
		edgeSet[edges[i].Sym()] = true
		vertexSet[edges[i].Orig()] = true
		vertexSet[edges[i].Dest()] = true
	}

	// verify the vertex index points to appropriate edges and vertices
	for v, e := range tri.vertexIndex {
		if _, ok := vertexSet[v]; ok == false {
			return fmt.Errorf("vertex index contains an unexpected vertex: %v", v)
		}
		if _, ok := edgeSet[e]; ok == false {
			if debug {
				log.Printf("subdiv: %v", tri.subdiv.DebugDumpEdges())
				for a, b := range edgeSet {
					log.Printf("%v %v", a, b)
				}
			}
			return fmt.Errorf("vertex index contains an unexpected edge: %v", e)
		}
		if v.Equals(e.Orig()) == false {
			return fmt.Errorf("vertex index points to an incorrect edge, expected %v got %v", e.Orig(), v)
		}
	}

	// verify all vertices are in the vertex index
	for v := range vertexSet {
		if _, ok := tri.vertexIndex[v]; ok == false {
			return fmt.Errorf("vertex index is missing a vertex: %v", v)
		}
	}

	return nil
}

/*
validateTriangles is a self consistency check that ensures all triangles are
valid.

If tri is nil a panic will occur.
*/
func (tri *Triangulator) validateTriangles() error {
	// collect a set of all edges
	edges := tri.subdiv.GetPrimaryEdges(true)
	for i := range edges {
		e := edges[i]
		t := Triangle{e}
		if t.IsValid() == false {
			for k, v := range tri.vertexIndex {
				if debug {
					log.Printf("k: %v v: %v", k, v)
				}
				e := v
				for {
					if debug {
						log.Printf("%v", e)
					}
					e = e.ONext()
					if e == v {
						break
					}
				}
			}
			return fmt.Errorf("Triangle is invalid: %v", &t)
		}
	}

	return nil
}
