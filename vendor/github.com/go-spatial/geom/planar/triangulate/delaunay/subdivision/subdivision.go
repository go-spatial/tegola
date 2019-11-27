package subdivision

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/go-spatial/geom/planar/intersect"
	"github.com/go-spatial/geom/winding"

	"github.com/gdey/errors"

	"github.com/go-spatial/geom/planar"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/quadedge"
)

const RoundingFactor = 1000

// Subdivision describes a quadedge graph that is used for triangulation
type Subdivision struct {
	startingEdge *quadedge.Edge
	ptcount      int
	frame        [3]geom.Point

	vertexIndexLock  sync.RWMutex
	vertexIndexCache VertexIndex
	Order            winding.Order
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

func newFromPointsTestCase(desc string, points [][2]float64, sd *Subdivision) {
	var strBuf strings.Builder

	fmt.Fprintf(&strBuf, `
TestCase:
	{ 
		Desc: "%v",
		Points: %#v,
	},

Desc: %v,
`, desc, points, desc)
	fmt.Fprintf(&strBuf, "Original Points: %v\n\n", wkt.MustEncode(geom.MultiPoint(points)))
	if sd != nil {
		fmt.Fprintf(&strBuf, "edges: %v", sd.startingEdge.DumpAllEdges())
	}
	log.Printf(strBuf.String())
}

// NewForPoints creates a new subdivision for the given points, the points are
// sorted and duplicate points are not added
func NewForPoints(ctx context.Context, points [][2]float64) (sd *Subdivision, err error) {
	//	if debug {
	defer func() {
		if err != nil && err != context.Canceled {
			newFromPointsTestCase(fmt.Sprintf("error:%v", err), points, sd)
		}
	}()
	//}

	for i := range points {
		points[i] = [2]float64(roundGeomPoint(geom.Point(points[i])))
	}

	tri := geom.NewTriangleContainingPoints(points...)
	sd = New(tri[0], tri[1], tri[2])

	if debug {
		if err := sd.Validate(ctx); err != nil {
			if err1, ok := err.(quadedge.ErrInvalid); ok {
				var strBuf strings.Builder
				fmt.Fprintf(&strBuf, "Invalid subdivision:\n Tri: %v\n", tri)
				for i, estr := range err1 {
					fmt.Fprintf(&strBuf, "\t%v : %v\n", i, estr)
				}
				log.Printf(strBuf.String())
				newFromPointsTestCase("invalid subdivision triangle", points, sd)
			}

			return sd, err
		} else {
			log.Printf("After new good")
		}
	}

	seen := make(map[geom.Point]bool)
	seen[tri[0]] = true
	seen[tri[1]] = true
	seen[tri[2]] = true

	for i, pt := range points {

		_ = i
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if seen[pt] {
			continue
		}
		seen[pt] = true

		if debug {
			log.Printf("\n\nInserting site %v: %v\n\n", i, wkt.MustEncode(pt))
		}

		if !sd.InsertSite(pt) {
			log.Printf("Failed to insert point(%v) %v", i, wkt.MustEncode(pt))
			return nil, errors.String("Failed to insert point")
		}
	}
	if debug {
		//	log.Printf("Validating Subdivision (%v of %v", i, len(points))
		if err := sd.Validate(ctx); err != nil {
			if err1, ok := err.(quadedge.ErrInvalid); ok {
				var strBuf strings.Builder
				fmt.Fprintf(&strBuf, "Invalid subdivision:\n")
				for i, estr := range err1 {
					fmt.Fprintf(&strBuf, "\t%v : %v\n", i, estr)
				}
				fmt.Fprintf(&strBuf, "Original Points: %v\n\n", wkt.MustEncode(geom.MultiPoint(points)))
				fmt.Fprintf(&strBuf, "edges: %v", sd.startingEdge.DumpAllEdges())
				log.Printf(strBuf.String())
				newFromPointsTestCase("invalid subdivision", points, sd)
			}

			return sd, err
		}
	}
	return sd, nil
}

// locate returns an edge e, s.t. either x is on e, or e is an edge of
// a triangle containing x. The search starts from startingEdge
// and proceeds in the general direction of x. Based on the
// pseudocode in Guibas and Stolfi (1985) p.121
func (sd *Subdivision) locate(x geom.Point) (*quadedge.Edge, bool) {
	return locate(sd.startingEdge, x, sd.ptcount*2)
}

// InsertSite will insert a new point into a subdivision representing a Delaunay
// triangulation, and fixes the affected edges so that the result
// is  still a Delaunay triangulation. This is based on the pseudocode
// from Guibas and Stolfi (1985) p.120, with slight modifications and a bug fix.
func (sd *Subdivision) InsertSite(x geom.Point) bool {

	if debug {
		log.Printf("\n\nInsertSite   %v  \n\n", wkt.MustEncode(x))
	}

	sd.ptcount++
	e, got := sd.locate(x)
	if !got {
		if debug {
			log.Println("did not find edge using normal walk")
		}
		// Did not find the edge using normal walk
		return false
	}
	if debug {
		log.Printf("insert %v found edge: %p %v", wkt.MustEncode(x), e, wkt.MustEncode(e.AsLine()))
		log.Printf("vertexes: %v", e.DumpAllEdges())
		log.Printf("subdivision")
		DumpSubdivision(sd)
	}

	if ptEqual(x, e.Orig()) || ptEqual(x, e.Dest()) {
		if debug {
			log.Printf("%v already in sd", wkt.MustEncode(x))
		}
		// Point is already in subdivision
		return true
	}

	// x should only be somewhere in the middle.
	if quadedge.OnEdge(x, e) {
		if debug {
			log.Printf("%v is on %v", wkt.MustEncode(x), wkt.MustEncode(e.AsLine()))
		}
		e = e.OPrev()

		// Check to see if this point is still already in subdivision.
		// not sure if this is needed
		if ptEqual(x, e.Orig()) || ptEqual(x, e.Dest()) {
			if debug {
				log.Printf("%v already in sd", wkt.MustEncode(x))
			}
			// Point is already in subdivision
			return true
		}
		if debug {
			log.Printf("removing %v", wkt.MustEncode(e.ONext().AsLine()))
		}

	}

	// Connect the new point to the vertices of the containing
	// triangle (or quadrilateral, if the new point fell on an
	// existing edge.)
	base := quadedge.NewWithEndPoints(e.Orig(), &x)
	if debug {
		log.Printf("Created new base: %v", wkt.MustEncode(base.AsLine()))
	}

	if debug {
		log.Printf("Splice base,e: %v", wkt.MustEncode(e.AsLine()))
	}
	quadedge.Splice(base, e)
	sd.startingEdge = base
	if debug {
		log.Printf("base edges: %v", base.DumpAllEdges())
		log.Printf("connecting e[ %v ] to base.Sym[ %v ]",
			wkt.MustEncode(e.AsLine()),
			wkt.MustEncode(base.Sym().AsLine()),
		)
	}

	base = quadedge.Connect(e, base.Sym(), sd.Order)
	// reset e
	e = base.OPrev()
	if debug {
		log.Printf("base: %v", wkt.MustEncode(base.AsLine()))
		log.Printf("base.OPrev/e: %v", wkt.MustEncode(e.AsLine()))
		log.Printf("e.LNext: %v", wkt.MustEncode(e.LNext().AsLine()))
		log.Printf("e.LPrev: %v", wkt.MustEncode(e.LPrev().AsLine()))
		log.Printf("e.RNext: %v", wkt.MustEncode(e.RNext().AsLine()))
		log.Printf("e.RPrev: %v", wkt.MustEncode(e.RPrev().AsLine()))
		log.Printf("se: %v", wkt.MustEncode(sd.startingEdge.AsLine()))
	}
	count := 0
	for e.LNext() != sd.startingEdge {
		if debug {
			log.Printf("connecting e[ %v ] to base.Sym[ %v ]",
				wkt.MustEncode(e.AsLine()),
				wkt.MustEncode(base.Sym().AsLine()),
			)
		}
		base = quadedge.Connect(e, base.Sym(), sd.Order)
		e = base.OPrev()
		if debug {
			count++
			log.Printf("se: %v", wkt.MustEncode(sd.startingEdge.AsLine()))
			log.Printf("base: %v", wkt.MustEncode(base.AsLine()))
			log.Printf("e == base.OPrev: %v", wkt.MustEncode(e.AsLine()))
			log.Printf("e.LNext: %v", wkt.MustEncode(e.LNext().AsLine()))
			log.Printf("subdivision: connect %v", count)
			if err := sd.Validate(context.Background()); err != nil {
				if err1, ok := err.(quadedge.ErrInvalid); ok {
					for i, estr := range err1 {
						log.Printf("err: %03v : %v", i, estr)
					}
				}
			} else {
				log.Printf("subdivision good")
			}
			DumpSubdivision(sd)
		}
	}
	if debug {
		log.Printf("Done adding edges, check for delaunay condition")
	}

	// Examine suspect edges to ensure that the Delaunay condition
	// is satisfied.
	for {
		t := e.OPrev()
		if debug {
			log.Printf("se: %v", wkt.MustEncode(sd.startingEdge.AsLine()))
			log.Printf("e: %v", wkt.MustEncode(e.AsLine()))
			log.Printf("e.OPrev/t: %v", wkt.MustEncode(t.AsLine()))
		}
		crl, err := geom.CircleFromPoints([2]float64(*e.Orig()), [2]float64(*t.Dest()), [2]float64(*e.Dest()))
		containsPoint := false
		if err == nil {
			containsPoint = crl.ContainsPoint([2]float64(x))
		}
		switch {
		case quadedge.RightOf(*t.Dest(), e) &&
			containsPoint:
			if debug {
				log.Printf("Circle from points: %v,%v,%v \n%v\n%v",
					wkt.MustEncode(*e.Orig()),
					wkt.MustEncode(*t.Dest()),
					wkt.MustEncode(*e.Dest()),
					wkt.MustEncode(crl.AsLineString(100)),
					containsPoint,
				)
				log.Printf("Point of consideration: %v", wkt.MustEncode(x))
				log.Printf("%v right of %v", wkt.MustEncode(*t.Dest()), wkt.MustEncode(e.AsLine()))
				log.Printf("Swapping e: %v", wkt.MustEncode(e.AsLine()))
			}
			quadedge.Swap(e)
			if debug {
				log.Printf("e: %v", wkt.MustEncode(e.AsLine()))
				log.Printf("e.OPrev: %v", wkt.MustEncode(e.OPrev().AsLine()))
			}
			e = e.OPrev()

		case e.ONext() == sd.startingEdge: // no more suspect edges
			if debug {
				if err := sd.Validate(context.Background()); err != nil {
					if err1, ok := err.(quadedge.ErrInvalid); ok {
						for i, estr := range err1 {
							log.Printf("err: %03v : %v", i, estr)
						}
					}
				} else {
					log.Printf("subdivision good")
				}
				DumpSubdivision(sd)
			}
			return true

		default: // pop a suspect edge
			e = e.ONext().LPrev()

		}
	}
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

// Triangles will return the triangles in the graph
func (sd *Subdivision) Triangles(includeFrame bool) (triangles [][3]geom.Point, err error) {

	if sd == nil {
		return nil, errors.String("subdivision is nil")
	}

	ctx := context.Background()
	WalkAllTriangles(ctx, sd.startingEdge, func(start, mid, end geom.Point) bool {
		if IsFramePoint(sd.frame, start, mid, end) && !includeFrame {
			return true
		}
		triangles = append(triangles, [3]geom.Point{start, mid, end})
		return true
	})

	return triangles, nil
}

// Validate will run a set of validation tests against the sd to insure
// the sd was built correctly. This process is very cpu and memory intensive
func (sd *Subdivision) Validate(ctx context.Context) error {

	var (
		lines []geom.Line
		err1  quadedge.ErrInvalid
	)

	if err := sd.WalkAllEdges(func(e *quadedge.Edge) error {
		l := e.AsLine()
		if err := quadedge.Validate(e, sd.Order); err != nil {
			if verr, ok := err.(quadedge.ErrInvalid); ok {
				wktStr, wktErr := wkt.EncodeString(l)
				if wktErr != nil {
					wktStr = wktErr.Error()
				}
				err1 = append(err1, fmt.Sprintf("edge: %v", wktStr))
				err1 = append(err1, verr...)
				return err1
			}
			return err
		}
		l2 := l.LengthSquared()
		if l2 == 0 {
			err1 = append(err1, "zero length edge: %v", wkt.MustEncode(l))
			return err1
		}
		lines = append(lines, l)
		return nil
	}); err != nil {
		return err
	}

	// Check for intersecting lines
	eq := intersect.NewEventQueue(lines)
	if err := eq.FindIntersects(ctx, true, func(i, j int, _ [2]float64) error {
		err1 = append(err1, fmt.Sprintf("found intersecting lines: \n%v\n%v", wkt.MustEncode(lines[i]), wkt.MustEncode(lines[j])))
		return err1
	}); err != nil {
		return err
	}

	return nil
}

// IsValid will walk the graph making sure it is in a valid state
func (sd *Subdivision) IsValid(ctx context.Context) bool { return sd.Validate(ctx) == nil }

//
//*********************************************************************************************************
//  VertexIndex
//*********************************************************************************************************
//

// VertexIndex is an index of points to an quadedge in the graph
// this allows one to quickly jump to a group of edges by the origin
// point of that edge
type VertexIndex map[geom.Point]*quadedge.Edge

// VertexIndex will calculate and return a VertexIndex that can be used to
// quickly look up vertexes
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
		orig = roundGeomPoint(*e.Orig())
		dest = roundGeomPoint(*e.Dest())
	)
	if _, ok = vx[orig]; !ok {
		vx[orig] = e
	}
	if _, ok = vx[dest]; !ok {
		vx[dest] = e.Sym()
	}
}

// Get retrives the edge
func (vx VertexIndex) Get(pt geom.Point) (*quadedge.Edge, bool) {
	pt = roundGeomPoint(pt)
	e, ok := vx[pt]
	return e, ok
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
		v = roundGeomPoint(v)
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

//
//*********************************************************************************************************
//  Helpers
//*********************************************************************************************************
//

// WalkAllEdges will call fn for each edge starting with se
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

// IsHardFrameEdge indicates if the edge is part of the given frame where both vertexes are part of the frame.
func IsHardFrameEdge(frame [3]geom.Point, e *quadedge.Edge) bool {
	o, d := *e.Orig(), *e.Dest()
	of := cmp.GeomPointEqual(o, frame[0]) || cmp.GeomPointEqual(o, frame[1]) || cmp.GeomPointEqual(o, frame[2])
	df := cmp.GeomPointEqual(d, frame[0]) || cmp.GeomPointEqual(d, frame[1]) || cmp.GeomPointEqual(d, frame[2])
	return of && df
}

// IsFramePoint indicates if at least one of the points is equal to one of the frame points
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

func WalkAllTriangles(ctx context.Context, se *quadedge.Edge, fn func(start, mid, end geom.Point) (shouldContinue bool)) {
	if se == nil || fn == nil {
		return
	}

	var (
		// Hold the edges we still have to look at
		edgeStack []*quadedge.Edge

		startingEdge *quadedge.Edge
		workingEdge  *quadedge.Edge
		nextEdge     *quadedge.Edge

		// Hold points we have already seen and can ignore
		seenVertices = make(map[geom.Point]bool)

		endPoint   geom.Point
		midPoint   geom.Point
		startPoint geom.Point

		count int
		loop  int
	)

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
		if seenVertices[startPoint] {
			// we have already processed this vertex
			continue
		}

		seenVertices[startPoint] = true

		workingEdge = startingEdge
		nextEdge = startingEdge.ONext()
		if workingEdge == nextEdge {
			continue
		}

		for {
			loop++
			endPoint = *nextEdge.Dest()
			midPoint = *workingEdge.Dest()
			if seenVertices[endPoint] || seenVertices[midPoint] {
				// we have already accounted for this triangle
				goto ADVANCE
			}

			// Add the working edge to the stack.
			edgeStack = append(edgeStack, workingEdge.Sym())

			if workingEdge.Sym().FindONextDest(endPoint) != nil && !fn(startPoint, midPoint, endPoint) {
				// found a triangle
				return
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

// FindIntersectingEdges will find all edges in the graph that would be intersected by the origin of the starting edge and the
// dest of the endingEdge
func FindIntersectingEdges(startingEdge, endingEdge *quadedge.Edge) (edges []*quadedge.Edge, err error) {
	/*
					 Move starting edge so that the graph look like
					 ◌ .
		 se.ONext()╱ ┆ nse.Sym().ONext()   | \  ee
				  ╱  ┆					   |  \
		 (start) ● r ┆ l                 l | r ◍ (end)
				  ╲  ┆	 nee.Sym().ONext() |  /
		 	 	 se╲ ┆					   | /  ee.ONext()
					                       ◌

		right face of se is the triangle face, we want
		to go left, to find the next shared edge till
		we get to shared edge.
	*/

	if debug {

		log.Printf("\n\n FindIntersectingEdges \n\n\n")
		log.Printf("starting, %p\n%v\n", startingEdge, wkt.MustEncode(startingEdge.AsLine()))
		log.Printf("starting:ONext:Sym, %p\n%v\n", startingEdge.ONext().Sym(), wkt.MustEncode(startingEdge.ONext().Sym().AsLine()))
		log.Printf("ending, %p\n%v\n", endingEdge, wkt.MustEncode(endingEdge.AsLine()))
		log.Printf("ending:ONext:Sym, %p\n%v\n", endingEdge.ONext().Sym(), wkt.MustEncode(endingEdge.ONext().Sym().AsLine()))

	}

	if startingEdge == nil || endingEdge == nil {
		return edges, nil
	}

	start, end := *startingEdge.Orig(), *endingEdge.Orig()
	line := geom.Line{[2]float64(start), [2]float64(end)}
	if debug {
		log.Printf("line,\n%v\n", wkt.MustEncode(line))
	}
	if line.LengthSquared() == 0 {
		// nothing to do
		return edges, nil
	}

	startingEdge, _ = quadedge.ResolveEdge(startingEdge, end)
	endingEdge, _ = quadedge.ResolveEdge(endingEdge, start)

	if cmp.GeomPointEqual(*startingEdge.Dest(), end) ||
		cmp.GeomPointEqual(*endingEdge.Dest(), start) {
		// the intersect lines already exists.
		return []*quadedge.Edge{}, nil
	}

	if debug {
		log.Printf("\n\nAfter Resolve\n\n")

		log.Printf("starting, %p\n%v\n", startingEdge, wkt.MustEncode(startingEdge.AsLine()))
		log.Printf("starting:ONext:Sym, %p\n%v\n", startingEdge.ONext().Sym(), wkt.MustEncode(startingEdge.ONext().Sym().AsLine()))
		log.Printf("ending, %p\n%v\n", endingEdge, wkt.MustEncode(endingEdge.AsLine()))
		log.Printf("ending:ONext:Sym, %p\n%v\n", endingEdge.ONext().Sym(), wkt.MustEncode(endingEdge.ONext().Sym().AsLine()))
		log.Printf("line,\n%v\n", wkt.MustEncode(line))
	}
	sharedSE := startingEdge.ONext().Sym().ONext()
	sharedEE := endingEdge.ONext().Sym().ONext()

	if debug {
		log.Printf("shared starting, %p\n%v\n", sharedSE, wkt.MustEncode(sharedSE.AsLine()))
		log.Printf("shared end, %p\n%v\n", sharedEE, wkt.MustEncode(sharedEE.AsLine()))
	}

	count := 0
	workingEdge := sharedSE

	if debug {
		log.Printf("\n\nEdges\n\n")
	}

	for {
		count++
		if count > 21 {
			log.Printf("Failing with infint loop")
			log.Printf("starting, %p\n%v\n", startingEdge, wkt.MustEncode(startingEdge.AsLine()))
			log.Printf("starting:ONext:Sym, %p\n%v\n", startingEdge.ONext().Sym(), wkt.MustEncode(startingEdge.ONext().Sym().AsLine()))
			log.Printf("ending, %p\n%v\n", endingEdge, wkt.MustEncode(endingEdge.AsLine()))
			log.Printf("ending:ONext:Sym, %p\n%v\n", endingEdge.ONext().Sym(), wkt.MustEncode(endingEdge.ONext().Sym().AsLine()))
			log.Printf("line,\n%v\n", wkt.MustEncode(line))
			return edges, fmt.Errorf("infint loop")
		}

		wln := workingEdge.AsLine()
		nwln := workingEdge.ONext().AsLine()

		if debug {
			log.Printf("%3v working, %p\n%v\n", count, workingEdge, wkt.MustEncode(wln))
			log.Printf("%3v working:ONext, %p\n%v\n", count, workingEdge.ONext(), wkt.MustEncode(nwln))
		}

		if _, intersected := planar.SegmentIntersect(line, wln); intersected {
			if debug {
				log.Println("adding working edge to list of edges")
			}
			edges = append(edges, workingEdge)
		}

		if sharedEE.IsEqual(workingEdge) {
			// We have reached the end
			return edges, nil
		}

		if ipt, intersected := planar.SegmentIntersect(line, nwln); intersected {
			workingEdge = workingEdge.ONext()
			wln = workingEdge.AsLine()
			if debug {
				log.Printf("onext wln intersects line: %v\n%v\n%v", wkt.MustEncode(nwln), wkt.MustEncode(line), ipt)
				log.Printf("\nGoing to ONext()\n")
				log.Printf("working, %p\n%v\n", workingEdge, wkt.MustEncode(wln))
			}
			continue
		}

		workingEdge = workingEdge.ONext().Sym().ONext()
		if debug {
			log.Printf("\nGoing to ONext().Sym().ONext()\n")
			log.Printf("working, %p\n%v\n", workingEdge, wkt.MustEncode(wln))
			log.Printf("working:ONext, %p\n%v\n", workingEdge.ONext(), wkt.MustEncode(nwln))
		}

	}
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
		if debug {
			log.Printf("%v right of  %v", wkt.MustEncode(x), wkt.MustEncode(e.AsLine()))
		}
		return e.Sym(), false

	case !quadedge.RightOf(x, e.ONext()):
		if debug {
			log.Printf("%v not right of  %v", wkt.MustEncode(x), wkt.MustEncode(e.ONext().AsLine()))
		}
		return e.ONext(), false

	case !quadedge.RightOf(x, e.DPrev()):
		if debug {
			log.Printf("%v not right of  %v", wkt.MustEncode(x), wkt.MustEncode(e.DPrev().AsLine()))
		}
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
		err   error
	)

	_ = err
	if debug {
		log.Printf("\n\nlocate\n\n")
		log.Printf("Original Starting Edge: %v", wkt.MustEncode(se.AsLine()))
	}
	se, err = quadedge.ResolveEdge(se, x)
	if debug {
		log.Printf("Starting Edge: %v : %v", wkt.MustEncode(se.AsLine()), err)
	}

	for e, ok = testEdge(x, se); !ok; e, ok = testEdge(x, e) {
		if debug {
			log.Printf("next Edge: %v", wkt.MustEncode(e.AsLine()))
		}
		if limit <= 0 {
			// don't care about count
			continue
		}

		count++
		if e == se || count > limit {
			e = nil

			WalkAllEdges(se, func(ee *quadedge.Edge) error {
				if _, ok = testEdge(x, ee); ok {
					e = ee
					return ErrCancelled
				}
				return nil
			})
			return e, false
		}
	}
	if debug {
		log.Printf("found Edge: %v\n\n", wkt.MustEncode(e.AsLine()))
	}
	return e, true

}
