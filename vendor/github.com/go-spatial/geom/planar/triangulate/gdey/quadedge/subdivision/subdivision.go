package subdivision

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/go-spatial/geom/planar"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/internal/debugger"
	"github.com/go-spatial/geom/planar/triangulate/gdey/quadedge/quadedge"
)

type VertexIndex map[geom.Point]*quadedge.Edge

type Subdivision struct {
	startingEdge *quadedge.Edge
	ptcount      int
	frame        [3]geom.Point
}

// New initialize a subdivision to the triangle defined by the points a,b,c.
func New(a, b, c geom.Point) *Subdivision {
	ea := quadedge.New()
	ea.EndPoints(&a, &b)
	eb := quadedge.New()
	quadedge.Splice(ea.Sym(), eb)
	eb.EndPoints(&b, &c)

	ec := quadedge.New()
	ec.EndPoints(&c, &a)
	quadedge.Splice(eb.Sym(), ec)
	quadedge.Splice(ec.Sym(), ea)
	return &Subdivision{
		startingEdge: ea,
		ptcount:      3,
		frame:        [3]geom.Point{a, b, c},
	}
}

func NewForPoints(ctx context.Context, points [][2]float64) *Subdivision {
	sort.Sort(cmp.ByXY(points))
	tri := geom.NewTriangleContainingPoints(points...)
	sd := New(tri[0], tri[1], tri[2])
	var oldPt geom.Point
	for i, pt := range points {
		if ctx.Err() != nil {
			return nil
		}
		bfpt := geom.Point(pt)
		if i != 0 && cmp.GeomPointEqual(oldPt, bfpt) {
			continue
		}
		oldPt = bfpt
		if !sd.InsertSite(bfpt) {
			log.Printf("Failed to insert point %v", bfpt)
		}
	}
	return sd
}

func ptEqual(x geom.Point, a *geom.Point) bool {
	if a == nil {
		return false
	}
	return cmp.GeomPointEqual(x, *a)
}

func testEdge(x geom.Point, e *quadedge.Edge) (*quadedge.Edge, bool) {
	switch {
	case ptEqual(x, e.Orig()) || ptEqual(x, e.Dest()):
		return e, true
	case quadedge.RightOf(x, e):
		return e.Sym(), false
	case !quadedge.RightOf(x, e.ONext()):
		return e.ONext(), false
	case !quadedge.RightOf(x, e.DPrev()):
		return e.DPrev(), false
	default:
		return e, true
	}
}

func locate(se *quadedge.Edge, x geom.Point, limit int) (*quadedge.Edge, bool) {
	var (
		e     *quadedge.Edge
		ok    bool
		count int
	)
	for e, ok = testEdge(x, se); !ok; e, ok = testEdge(x, e) {
		if limit > 0 {

			count++
			if e == se || count > limit {
				log.Println("searching all edges for", x)
				e = nil

				WalkAllEdges(se, func(ee *quadedge.Edge) error {
					if _, ok = testEdge(x, ee); ok {
						e = ee
						return ErrCancelled
					}
					return nil
				})
				log.Printf(
					"Got back to starting edge after %v iterations, only have %v points ",
					count,
					limit,
				)
				return e, false
			}
		}
	}
	return e, true

}

// VertexIndex will calculate and return a VertexIndex that can be used to
// quickly look up vertexies
func (sd *Subdivision) VertexIndex() VertexIndex {
	return NewVertexIndex(sd.startingEdge)
}

// NewVertexIndex will return a new vertex index given a starting edge.
func NewVertexIndex(startingEdge *quadedge.Edge) VertexIndex {
	vx := make(VertexIndex)
	WalkAllEdges(startingEdge, func(e *quadedge.Edge) error {
		vx.Add(e)
		return nil
	})
	return vx
}

// Add an edge to the graph
func (vx VertexIndex) Add(e *quadedge.Edge) {
	var (
		ok   bool
		orig = *e.Orig()
		dest = *e.Dest()
	)
	if _, ok = vx[orig]; !ok {
		vx[orig] = e
	}
	if _, ok = vx[dest]; !ok {
		vx[dest] = e.Sym()
	}
}

// Remove an edge from the graph
func (vx VertexIndex) Remove(e *quadedge.Edge) {
	// Don't think I need e.Rot() and e.Rot().Sym() in this list
	// as they are face of the quadedge.
	toRemove := [4]*quadedge.Edge{e, e.Sym(), e.Rot(), e.Rot().Sym()}
	shouldRemove := func(e *quadedge.Edge) bool {
		for i := range toRemove {
			if toRemove[i] == e {
				return true
			}
		}
		return false
	}

	for _, v := range [...]geom.Point{*e.Orig(), *e.Dest()} {
		ve := vx[v]
		if ve == nil || !shouldRemove(ve) {
			continue
		}
		delete(vx, v)
		// See if the ccw edge is the same as us, if it's isn't
		// then use that as the edge for our lookup.
		if ve != ve.ONext() {
			vx[v] = ve.ONext()
		}
	}
}

// locate returns an edge e, s.t. either x is on e, or e is an edge of
// a triangle containing x. The search starts from startingEdge
// and proceeds in the general direction of x. Based on the
// pseudocode in Guibas and Stolfi (1985) p.121
func (sd *Subdivision) locate(x geom.Point) (*quadedge.Edge, bool) {
	return locate(sd.startingEdge, x, sd.ptcount*2)
}

// FindEdge will iterate the graph looking for the edge
func (sd *Subdivision) FindEdge(vertexIndex VertexIndex, start, end geom.Point) *quadedge.Edge {
	if vertexIndex == nil {
		vertexIndex = sd.VertexIndex()
	}
	return vertexIndex[start].FindONextDest(end)
}

// InsertSite will insert a new point into a subdivision representing a Delaunay
// triangulation, and fixes the affected edges so that the result
// is  still a Delaunay triangulation. This is based on the pseudocode
// from Guibas and Stolfi (1985) p.120, with slight modificatons and a bug fix.
func (sd *Subdivision) InsertSite(x geom.Point) bool {
	sd.ptcount++
	e, got := sd.locate(x)
	if !got {
		// Did not find the edge using normal walk
		return false
	}

	if ptEqual(x, e.Orig()) || ptEqual(x, e.Dest()) {
		// Point is already in subdivision
		return true
	}

	if quadedge.OnEdge(x, e) {
		e = e.OPrev()
		// Check to see if this point is still alreayd there.
		if ptEqual(x, e.Orig()) || ptEqual(x, e.Dest()) {
			// Point is already in subdivision
			return true
		}
		quadedge.Delete(e.ONext())
	}

	// Connect the new point to the vertices of the containing
	// triangle (or quadrilaterial, if the new point fell on an
	// existing edge.)
	base := quadedge.NewWithEndPoints(e.Orig(), &x)
	quadedge.Splice(base, e)
	sd.startingEdge = base

	base = quadedge.Connect(e, base.Sym())
	e = base.OPrev()
	for e.LNext() != sd.startingEdge {
		base = quadedge.Connect(e, base.Sym())
		e = base.OPrev()
	}

	// Examine suspect edges to ensure that the Delaunay condition
	// is satisfied.
	for {
		t := e.OPrev()
		switch {
		case quadedge.RightOf(*t.Dest(), e) &&
			x.WithinCircle(*e.Orig(), *t.Dest(), *e.Dest()):
			quadedge.Swap(e)
			e = e.OPrev()

		case e.ONext() == sd.startingEdge: // no more suspect edges
			return true

		default: // pop a suspect edge
			e = e.ONext().LPrev()

		}
	}
}

func appendNonrepeat(pts []geom.Point, v geom.Point) []geom.Point {
	if len(pts) == 0 || cmp.GeomPointEqual(v, pts[len(pts)-1]) {
		return pts
	}
	return append(pts, v)
}

func selectCorrectEdges(from, to *quadedge.Edge) (cfrom, cto *quadedge.Edge) {
	orig := *from.Orig()
	dest := *to.Orig()
	cfrom, cto = from, to
	if debug {
		log.Printf("curr RightOf(dest)? %v", quadedge.RightOf(dest, cfrom))
		log.Printf("destedge.Sym RightOf(orig)? %v", quadedge.RightOf(orig, cto))
	}
	if !quadedge.RightOf(dest, cfrom) {
		cfrom = cfrom.OPrev()
	}
	if !quadedge.RightOf(orig, cto) {
		cto = cto.OPrev()
	}
	return cfrom, cto
}

func resolveEdge(gse *quadedge.Edge, dest geom.Point) *quadedge.Edge {

	if gse == nil {
		return nil
	}

	if debug {
		log.Printf("resolveEdge: starting at %v : %v", gse.AsLine(), debugger.FFL(0))
	}

	workingEdge := gse
	leftEdge := gse.OPrev()

	// only one or two edges... working edges is good enough
	if workingEdge == leftEdge {
		return gse
	}

	// only has two edges need to find the left one
	if workingEdge.ONext() == leftEdge {
		if quadedge.Classify(*workingEdge.Orig(), dest, *workingEdge.Dest()) == quadedge.LEFT {
			return workingEdge
		}
		return leftEdge
	}

	for {
		if debug {
			wln := workingEdge.AsLine()
			nln := leftEdge.AsLine()
			log.Printf(
				"Looking \n\t %v\n\t %v ",
				wkt.MustEncode(geom.MultiLineString{wln[:], nln[:]}),
				wkt.MustEncode(dest),
			)
			log.Printf(
				"dest: %v rightOf: %v leftOf: %v",
				dest,
				quadedge.RightOf(dest, workingEdge),
				quadedge.LeftOf(dest, leftEdge),
			)
		}

		classWorkingEdge := quadedge.Classify(*workingEdge.Orig(), dest, *workingEdge.Dest())
		classLeftEdge := quadedge.Classify(*leftEdge.Orig(), dest, *leftEdge.Dest())
		if classLeftEdge == quadedge.LEFT && classWorkingEdge == quadedge.RIGHT {
			return leftEdge
		}

		workingEdge = leftEdge
		leftEdge = workingEdge.OPrev()

		if workingEdge == gse {
			if debug {
				log.Printf("This should not happen")
			}
			return gse
		}
	}
}

func findImmediateRightOfEdges(se *quadedge.Edge, dest geom.Point) (*quadedge.Edge, *quadedge.Edge) {

	// We want the edge immediately left of the dest.

	orig := *se.Orig()
	if debug {
		log.Printf("Looking for orig fo %v to dest of %v", orig, dest)
	}
	curr := se
	for {
		if debug {
			log.Printf("top level looking at: %p (%v -> %v)", curr, *curr.Orig(), *curr.Dest())
		}
		if cmp.GeomPointEqual(*curr.Dest(), dest) {
			// edge already in the system.
			if debug {
				log.Printf("Edge already in system: %p", curr)
			}
			return curr, nil

		}

		// Need to see if the dest Next has the dest.
		for destedge := curr.Sym().ONext(); destedge != curr.Sym(); destedge = destedge.ONext() {
			if debug {
				log.Printf("\t looking at: %p (%v -> %v)", destedge, *destedge.Orig(), *destedge.Dest())
			}
			if cmp.GeomPointEqual(*destedge.Dest(), dest) {
				// found what we are looking for.
				if debug {
					log.Printf("Found the dest! %v -- %p %p", dest, curr, destedge.Sym())
				}

				return selectCorrectEdges(curr, destedge.Sym())
			}
			//log.Println("Next:", *destedge.Orig(), *curr.Sym().Orig(), *curr.Sym().Dest())

		}
		curr = curr.ONext()
		if curr == se {
			break
		}
	}
	return nil, nil
}

// WalkAllEdges will call the provided function for each edge in the subdivision. The walk will
// be terminated if the function returns an error or ErrCancel. ErrCancel will not result in
// an error be returned by main function, otherwise the error will be passed on.
func (sd *Subdivision) WalkAllEdges(fn func(e *quadedge.Edge) error) error {

	if sd == nil || sd.startingEdge == nil {
		return nil
	}
	return WalkAllEdges(sd.startingEdge, fn)
}

func (sd *Subdivision) Triangles(includeFrame bool) (triangles [][3]geom.Point, err error) {

	ctx := context.Background()
	WalkAllTriangles2(ctx, sd.startingEdge, func(start, mid, end geom.Point) bool {
		if IsFramePoint(sd.frame, start, mid, end) && !includeFrame {
			return true
		}
		triangles = append(triangles, [3]geom.Point{start, mid, end})
		return true
	})

	/*
		err = WalkAllTriangleEdges(
			sd.startingEdge,
			func(edges []*quadedge.Edge) error {
				if len(edges) != 3 {
					// skip this edge
					if debug {
						for i, e := range edges {
							log.Printf("got the following edge%v : %v", i, wkt.MustEncode(e.AsLine()))
						}
					}
					return nil
					//	return errors.New("Something Strange!")
				}

				pts := [3]geom.Point{*edges[0].Orig(), *edges[1].Orig(), *edges[2].Orig()}

				// Do we want to skip because the points are part of the frame and
				// we have been requested not to include triangles attached to the frame.
				if IsFramePoint(sd.frame, pts[:]...) && !includeFrame {
					return nil
				}

				triangles = append(triangles, pts)
				return nil
			},
		)
	*/
	return triangles, nil
}

func WalkAllEdges(se *quadedge.Edge, fn func(e *quadedge.Edge) error) error {
	if se == nil {
		return nil
	}
	var (
		toProcess quadedge.Stack
		visited   = make(map[*quadedge.Edge]bool)
	)
	toProcess.Push(se)
	for toProcess.Length() > 0 {
		e := toProcess.Pop()
		if visited[e] {
			continue
		}

		if err := fn(e); err != nil {
			if err == ErrCancelled {
				return nil
			}
			return err
		}

		sym := e.Sym()

		toProcess.Push(e.ONext())
		toProcess.Push(sym.ONext())

		visited[e] = true
		visited[sym] = true
	}
	return nil
}

// IsFrameEdge indicates if the edge is part of the given frame.
func IsFrameEdge(frame [3]geom.Point, es ...*quadedge.Edge) bool {
	for _, e := range es {
		o, d := *e.Orig(), *e.Dest()
		of := cmp.GeomPointEqual(o, frame[0]) || cmp.GeomPointEqual(o, frame[1]) || cmp.GeomPointEqual(o, frame[2])
		df := cmp.GeomPointEqual(d, frame[0]) || cmp.GeomPointEqual(d, frame[1]) || cmp.GeomPointEqual(d, frame[2])
		if of || df {
			return true
		}
	}
	return false
}

// IsFrameEdge indicates if the edge is part of the given frame where both vertexs are part of the frame.
func IsHardFrameEdge(frame [3]geom.Point, e *quadedge.Edge) bool {
	o, d := *e.Orig(), *e.Dest()
	of := cmp.GeomPointEqual(o, frame[0]) || cmp.GeomPointEqual(o, frame[1]) || cmp.GeomPointEqual(o, frame[2])
	df := cmp.GeomPointEqual(d, frame[0]) || cmp.GeomPointEqual(d, frame[1]) || cmp.GeomPointEqual(d, frame[2])
	return of && df
}

func IsFramePoint(frame [3]geom.Point, pts ...geom.Point) bool {
	for _, pt := range pts {
		if cmp.GeomPointEqual(pt, frame[0]) ||
			cmp.GeomPointEqual(pt, frame[1]) ||
			cmp.GeomPointEqual(pt, frame[2]) {
			return true
		}
	}
	return false

}

func constructTriangleEdges(
	e *quadedge.Edge,
	toProcess *quadedge.Stack,
	visited map[*quadedge.Edge]bool,
	fn func(edges []*quadedge.Edge) error,
) error {

	if visited[e] {
		return nil
	}

	curr := e
	var triedges []*quadedge.Edge
	for backToStart := false; !backToStart; backToStart = curr == e {

		// Collect edge
		triedges = append(triedges, curr)

		sym := curr.Sym()
		if !visited[sym] {
			toProcess.Push(sym)
		}

		// mark edge as visted
		visited[curr] = true

		// Move the ccw edge
		curr = curr.LNext()
	}
	return fn(triedges)
}

// WalkAllTriangleEdges will walk the subdivision starting from the starting edge (se) and return
// sets of edges that make make a triangle for each face.
func WalkAllTriangleEdges(se *quadedge.Edge, fn func(edges []*quadedge.Edge) error) error {
	if se == nil {
		return nil
	}
	var (
		toProcess quadedge.Stack
		visited   = make(map[*quadedge.Edge]bool)
	)
	toProcess.Push(se)
	for toProcess.Length() > 0 {
		e := toProcess.Pop()
		if visited[e] {
			continue
		}
		err := constructTriangleEdges(e, &toProcess, visited, fn)
		if err != nil {
			if err == ErrCancelled {
				return nil
			}
			return err
		}
	}
	return nil
}

func WalkAllTriangles2(ctx context.Context, se *quadedge.Edge, fn func(start, mid, end geom.Point) (shouldContinue bool)) {
	if se == nil || fn == nil {
		return
	}
	var rcd debugger.Recorder

	if debug {
		rcd = debugger.GetRecorderFromContext(ctx)
	}

	var (
		// Hold the edges we still have to look at
		edgeStack []*quadedge.Edge

		startingEdge *quadedge.Edge
		workingEdge  *quadedge.Edge
		nextEdge     *quadedge.Edge

		// Hold points we have already seen and can ignore
		seenVerticies = make(map[geom.Point]bool)

		endPoint   geom.Point
		midPoint   geom.Point
		startPoint geom.Point

		count int
		loop  int
	)
	if debug {
		debugger.RecordOn(rcd, se.AsLine(), "WalkAllTriangles", "starting edge %v", se.AsLine())
	}

	edgeStack = append(edgeStack, se)

	for len(edgeStack) > 0 {
		if debug {
			count++
			loop = 0
		}

		// Pop of an edge to process
		startingEdge = edgeStack[len(edgeStack)-1]
		edgeStack = edgeStack[:len(edgeStack)-1]
		startPoint = *startingEdge.Orig()
		if seenVerticies[startPoint] {
			if debug {
				debugger.RecordOn(rcd, startPoint, "WalkAllTriangles:SkipVertex", "count:%v loop:%v vertex:%v", count, loop, startPoint)
			}
			// we have already processed this vertix
			continue
		}

		seenVerticies[startPoint] = true
		debugger.RecordOn(rcd, startPoint, "WalkAllTriangles:Vertex", "count:%v loop:%v vertex:%v", count, loop, startPoint)

		workingEdge = startingEdge
		nextEdge = startingEdge.ONext()
		if workingEdge == nextEdge {
			if debug {
				debugger.RecordOn(rcd, workingEdge.AsLine(), "WalkAllTriangles:SkipEdge:work==next", "count:%v loop:%v edge:%v", count, loop, workingEdge.AsLine())
			}
			continue
		}

		for {
			loop++
			endPoint = *nextEdge.Dest()
			midPoint = *workingEdge.Dest()
			if debug {
				debugger.RecordOn(
					rcd,
					geom.MultiPoint{
						[2]float64(startPoint),
						[2]float64(midPoint),
						[2]float64(endPoint),
					},
					"WalkAllTriangles:Vertex:Initial", "count:%v loop:%v initial verticies", count, loop,
				)
				debugger.RecordOn(
					rcd,
					geom.Triangle{
						[2]float64(startPoint),
						[2]float64(midPoint),
						[2]float64(endPoint),
					},
					"WalkAllTriangles:Triangle:Initial", "count:%v loop:%v prospective triangle", count, loop,
				)
				wln := workingEdge.AsLine()
				nln := nextEdge.AsLine()
				debugger.RecordOn(
					rcd,
					geom.MultiLineString{
						wln[:],
						nln[:],
					},
					"WalkAllTriangles:Edge:Initial", "count:%v loop:%v initial edges", count, loop,
				)
			}
			if seenVerticies[endPoint] || seenVerticies[midPoint] {
				if debug {
					skipPoint := midPoint
					if seenVerticies[endPoint] {
						skipPoint = endPoint
					}

					debugger.RecordOn(rcd, skipPoint, "WalkAllTriangles:SkipTriangle", "count:%v loop:%v vertex:%v(%v),%v(%v)", count, loop, midPoint, seenVerticies[midPoint], endPoint, seenVerticies[endPoint])
				}
				// we have already accounted for this triangle
				goto ADVANCE
			}

			// Add the working edge to the stack.
			edgeStack = append(edgeStack, workingEdge.Sym())
			if debug {
				debugger.RecordOn(rcd, workingEdge.AsLine(), "WalkAllTriangles:Edge", "count:%v loop:%v work-edge:%v", count, loop, workingEdge.AsLine())
			}

			if workingEdge.Sym().FindONextDest(endPoint) != nil {
				// found a triangle
				// *workingEdge.Orig(),*workingEdge.Dest(), *nextEdge.Dest()
				if debug {
					tri := geom.Triangle{[2]float64(startPoint), [2]float64(midPoint), [2]float64(endPoint)}
					debugger.RecordOn(rcd, tri, "WalkAllTriangles:Triangle", "count:%v loop:%v triangle:%v", count, loop, tri)
				}
				if !fn(startPoint, midPoint, endPoint) {
					return
				}
			} else if debug {
				debugger.RecordOn(rcd, endPoint, "WalkAllTriangles:Vertex", "count:%v loop:%v endPoint:%v not connected", count, loop, endPoint)
				debugger.RecordOn(rcd, workingEdge.Sym().AsLine(), "WalkAllTriangles:Edge", "count:%v loop:%v work-edge-sym:%v not connected", count, loop, workingEdge.Sym().AsLine())
				debugger.RecordOn(rcd, nextEdge.AsLine(), "WalkAllTriangles:Edge", "count:%v loop:%v next-edge:%v not connected", count, loop, nextEdge.AsLine())
			}

		ADVANCE:
			workingEdge = nextEdge
			nextEdge = workingEdge.ONext()
			if workingEdge == startingEdge {
				break
			}
		}

	}
}

func FindIntersectingEdges2(startingEdge *quadedge.Edge, end geom.Point) (edges []*quadedge.Edge) {

	start := *startingEdge.Orig()
	line := geom.Line{[2]float64(start), [2]float64(end)}
	seenIntersectingEdges := make(map[*quadedge.Edge]bool)

	edgeCheck := func(edge *quadedge.Edge) bool {
		// check to see if it intersects
		_, intersected := planar.SegmentIntersect(line, edge.AsLine())
		if !intersected {
			return false
		}
		if seenIntersectingEdges[edge] {
			return false
		}
		if cmp.GeomPointEqual(end, *edge.Orig()) || cmp.GeomPointEqual(end, *edge.Dest()) {
			return false
		}
		if cmp.GeomPointEqual(start, *edge.Orig()) || cmp.GeomPointEqual(start, *edge.Dest()) {
			return false
		}
		return true
	}

	/*
		 Move starting edge so that the graph look like

		             ◌
				se ╱ ┆ se.Sym().ONext()
			      ╱  ┆
				 ●...┆..........◍ (end)
				  ╲  ┆
		 se.OPrev()╲ ┆
					 ◌
	*/
	startingEdge = resolveEdge(startingEdge, end)
	workingEdge := startingEdge.Sym().ONext()

	for {
		edges = append(edges, workingEdge)
		seenIntersectingEdges[workingEdge] = true
		/*

			Now I have four possible edges to check.
			workingEdge.ONext()
			workingEdge.OPrev()
			workingEdge.Sym().ONext()
			workingEdge.Sym().OPrev()
			The first one that intersect,
			 but we have not see before,
			 and the end-points are not equal to end
			 is the next working edge.
		*/
		if edgeCheck(workingEdge.ONext()) {
			workingEdge = workingEdge.ONext()
			continue
		}
		if edgeCheck(workingEdge.OPrev()) {
			workingEdge = workingEdge.OPrev()
			continue
		}
		if edgeCheck(workingEdge.Sym().ONext()) {
			workingEdge = workingEdge.Sym().ONext()
			continue
		}
		if edgeCheck(workingEdge.Sym().OPrev()) {
			workingEdge = workingEdge.Sym().OPrev()
			continue
		}
		break
	}
	return edges
}

func FindIntersectingEdges(startingEdge, endingEdge *quadedge.Edge) (edges []*quadedge.Edge, err error) {

	/*
					 Move starting edge so that the graph look like
					 ◌ .
				se ╱ ┆ se.Sym().ONext() .  | \  ee.OPrev()
				  ╱  ┆					   |  \
		 (start) ● r ┆ l                 l | r ◍ (end)
				  ╲  ┆	  ee.Sym().ONext() |  /
		 se.OPrev()╲ ┆					   | /  ee
					                       ◌

		right face of se is the triangle face, we want
		to go left, to find the next shared edge till
		we get to shared edge.
	*/

	if startingEdge == nil || endingEdge == nil {
		return edges, nil
	}

	start, end := *startingEdge.Orig(), *endingEdge.Orig()
	line := geom.Line{[2]float64(start), [2]float64(end)}

	startingEdge = resolveEdge(startingEdge, end)
	endingEdge = resolveEdge(endingEdge, start)
	log.Printf("starting, %p\n%v\n", startingEdge, wkt.MustEncode(startingEdge.AsLine()))
	log.Printf("starting:OPrev, %p\n%v\n", startingEdge.OPrev(), wkt.MustEncode(startingEdge.OPrev().AsLine()))
	log.Printf("end, %p\n%v\n", endingEdge, wkt.MustEncode(endingEdge.AsLine()))
	log.Printf("end:OPrev, %p\n%v\n", endingEdge.OPrev(), wkt.MustEncode(endingEdge.OPrev().AsLine()))
	log.Printf("line,\n%v\n", wkt.MustEncode(line))

	sharedSE := startingEdge.Sym().ONext()
	sharedEE := endingEdge.Sym().ONext()
	log.Printf("shared starting, %p\n%v\n", sharedSE, wkt.MustEncode(sharedSE.AsLine()))
	log.Printf("shared end, %p\n%v\n", sharedEE, wkt.MustEncode(sharedEE.AsLine()))
	log.Printf("shared end opp, \n%v\n", wkt.MustEncode(endingEdge.Sym().OPrev().AsLine()))

	count := 0
	workingEdge := sharedSE

	log.Println("Edges")

	for {
		count++
		if count > 11 {
			return edges, fmt.Errorf("infint loop")
		}

		/*
			// check to see if the working edge has the
			// dest endpoint
			if cmp.GeomPointEqual(end, *workingEdge.Orig()) || cmp.GeomPointEqual(end, *workingEdge.Dest()) {
				log.Println("workingedge has endpoint")
				return edges, nil
			}
		*/

		edges = append(edges, workingEdge)
		if sharedEE.IsEqual(workingEdge) {
			// We have reached the end
			return edges, nil
		}

		log.Printf("%v     , %p\n%v\n", count, workingEdge, wkt.MustEncode(workingEdge.AsLine()))
		log.Printf("%v:next, %p\n%v\n", count, workingEdge.LNext(), wkt.MustEncode(workingEdge.LNext().AsLine()))
		log.Printf("%v:prev, %p\n%v\n", count, workingEdge.LPrev(), wkt.MustEncode(workingEdge.LPrev().AsLine()))

		// look at LNext() and LPrev()
		// first to see if it is equal to sharedEE
		// if it is we are done
		// otherwise we need to see it intersects with
		// our line
		tedge := workingEdge.LNext().Sym()

		// check to see if the line intersects
		_, intersected := planar.SegmentIntersect(line, tedge.AsLine())
		if intersected {
			log.Println("LNext() matched")
			workingEdge = tedge
			continue
		}

		// As we are assuming we are in a triangle we know
		// a face can only have three edges
		workingEdge = workingEdge.ONext()
	}
}

func FindIntersectingEdges3(startingEdge *quadedge.Edge, end geom.Point) (edges []*quadedge.Edge, err error) {

	startingEdge = resolveEdge(startingEdge, end)

	workingEdge := startingEdge
	for {
		if workingEdge.FindONextDest(end) != nil {
			// the wrokingEdge.Orig -> end exists. let's flip our line
			workingEdge = workingEdge.Sym()
			if workingEdge.FindONextDest(end) != nil {
				// both edges of the triangle lead to the end point
				return edges, nil
			}
		}

		//vpts = append(vpts, *workingEdge.Orig(), *workingEdge.Dest())

		// Assumption edge (startingEdge.Orig(), end) does not exist in the graph
		workingEdge = resolveEdge(workingEdge, end)
		/*
						 Move starting edge so that the graph look like
			                 ◌
						se ╱ ┆ se.Sym().ONext()
					      ╱  ┆
						 ●   ┆          ◍ (end)
						  ╲  ┆
				 se.OPrev()╲ ┆
				             ◌
		*/
		workingEdge = workingEdge.Sym().ONext()
		if len(edges) > 4 || (len(edges) > 0 && workingEdge == edges[0]) {
			log.Println(edges)
			for _, e := range edges {
				log.Println(wkt.MustEncode(e.AsLine()))
			}

			return nil, fmt.Errorf("inifite loop")
		}
		edges = append(edges, workingEdge)
	}
}

func FindIntersectingTriangle(startingEdge *quadedge.Edge, end geom.Point) (*Triangle, error) {
	var (
		left  = startingEdge
		right *quadedge.Edge
	)

	for {
		right = left.OPrev()

		lc := quadedge.Classify(end, *left.Orig(), *left.Dest())
		rc := quadedge.Classify(end, *right.Orig(), *right.Dest())

		if (lc == quadedge.RIGHT && rc == quadedge.LEFT) ||
			lc == quadedge.BETWEEN ||
			lc == quadedge.DESTINATION ||
			lc == quadedge.BEYOND {
			return &Triangle{left}, nil
		}

		if lc != quadedge.RIGHT && lc != quadedge.LEFT &&
			rc != quadedge.RIGHT && rc != quadedge.LEFT {
			return &Triangle{left}, ErrCoincidentalEdges
		}
		left = right
		if left == startingEdge {
			// We have walked all around the vertex.
			break
		}

	}
	return nil, nil
}

// IsValid will walk the graph making sure it is in a valid state
func (sd *Subdivision) IsValid(ctx context.Context) bool {
	count := 0
	valid := true
	if cgo && debug {

		ctx = debugger.AugmentContext(ctx, "")
		defer debugger.Close(ctx)

	}
	_ = sd.WalkAllEdges(func(e *quadedge.Edge) error {
		l := e.AsLine()
		if err := quadedge.Validate(e); err != nil {
			log.Printf("%v not valid", l)
			errs, _ := err.(quadedge.ErrInvalid)
			for _, str := range errs {
				log.Println("error\n", str)
			}
			valid = false
			return context.Canceled
		}
		l2 := l.LenghtSquared()
		if l2 == 0 {
			count++
			if debug {
				debugger.Record(ctx,
					l,
					"ZeroLenght:Edge",
					"Line (%p) %v -- %v ", e, l2, l,
				)
			}
		}
		return nil
	})
	if debug {
		log.Println("Count", count)
	}
	return valid && count == 0
}
