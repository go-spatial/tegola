package subdivision

import (
	"log"
	"math"
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/planar"
)

type edgeMap map[geom.Line]bool

func (em edgeMap) Contains(p1, p2 geom.Point) bool {
	ln := geom.Line{p1, p2}
	normalizeLine(&ln)
	return em[ln]
}
func (em edgeMap) AddEdge(ln geom.Line) {
	normalizeLine(&ln)
	em[ln] = true
}

func (em edgeMap) Edges() (lns []geom.Line) {
	lns = make([]geom.Line, 0, len(em))
	for ln := range em {
		lns = append(lns, ln)
	}
	sort.Sort(sort.Reverse(planar.LinesByLength(lns)))
	return lns
}

func normalizeLine(ln *geom.Line) {
	if cmp.PointLess(ln[0], ln[1]) {
		ln[0], ln[1] = ln[1], ln[0]
	}
}
func newEdgeMap(points []geom.Point) edgeMap {

	em := make(edgeMap)
	lp := len(points) - 1
	for i := range points {
		em.AddEdge(geom.Line{points[lp], points[i]})
		lp = i
	}
	return em
}

// triangulatePseudoPolygon will return triangulated edges for the given polygon. The edges are not
// guaranteed to be unique or normalized.
func triangulatePseudoPolygon(oPoints []geom.Point) (edges []geom.Line, err error) {

	if debug {
		log.Printf("Step-1: starting points(%v): %v", len(oPoints), wkt.MustEncode(oPoints))
	}
	points := make([]geom.Point, 0, len(oPoints))

	// Let remove duplicate points.
	{
		seenPts := make(map[geom.Point]bool, len(oPoints))
		for _, pt := range oPoints {
			if seenPts[pt] {
				continue
			}
			seenPts[pt] = true
			points = append(points, pt)
		}
		if debug {
			log.Printf("Step 0: starting points(%v): %v", len(points), wkt.MustEncode(points))
		}
		points = points[0:len(points):len(points)]
	}
	if debug {
		log.Printf("Step 0: starting points(%v): %v", len(points), wkt.MustEncode(points))
	}

	plen := len(points)

	if plen <= 1 {
		return nil, ErrInvalidPseudoPolygonSize
	}
	if plen == 2 {
		return []geom.Line{
			geom.Line{points[0], points[1]},
		}, nil
	}

	if plen == 3 {
		return []geom.Line{
			geom.Line{points[0], points[1]},
			geom.Line{points[1], points[2]},
			geom.Line{points[2], points[0]},
		}, nil
	}

	em := newEdgeMap(points)

	// find the next two closes points to the center of the line between the
	// first and last points.
	x1, y1, x2, y2 := points[0][0], points[0][1], points[plen-1][0], points[plen-1][1]
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	cpoint := geom.Point{(x2 - x1) / 2, (y2 - y1) / 2}
	dist := math.Inf(1)
	var ps, p1, p2, pe int = 0, -1, -1, plen - 1

	for i, candidate := range points[1:pe] {
		d := planar.PointDistance2(cpoint, candidate)
		if d < dist {
			p1, p2, dist = i+1, p1, d
		}
	}

	if debug {
		log.Printf("Step 1: Line to point: %v", wkt.MustEncode(geom.Line{points[0], points[plen-1]}))
		log.Printf("Step 1: Found closes point: %v ", wkt.MustEncode(points[p1]))
	}
	jumpOffset := 1
	if p2 == -1 {
		p2 = p1 - jumpOffset
	}
	if debug {
		log.Printf("Step 1a: Found next closes point: %v ", wkt.MustEncode(points[p2]))
	}

ColinearTest:
	if p2 < 1 { // don't want p2 to equal the start or end points
		p2 = p1 + jumpOffset
	}
	if debug {
		log.Println("Looking at the following pts:", ps, p1, p2, pe)
		log.Println("\tpoints:", wkt.MustEncode([]geom.Point{points[ps], points[p1], points[p2], points[pe]}))
	}

	if geom.IsColinear(points[p1], points[ps], points[pe]) {
		p1, p2 = p2, p1
		// Let's check again, if it colinear we need a new point
		if geom.IsColinear(points[p1], points[ps], points[pe]) {
			// the points are still colinear, and we don't have anymore points to pick
			if plen == 4 || jumpOffset == plen {
				return nil, geom.ErrPointsAreCoLinear
			}
			jumpOffset++
			// p2 is currently the old p1; restore and try a different point
			p1, p2 = p2, p2-jumpOffset
			goto ColinearTest
		}
	}

	// we now have the last two closes point to the center.
	//                a ← ps
	//               / \
	//              /   \
	//             /     \
	//       pe → b-------c ← p1
	//             \     /
	//              \   /
	//               \ /
	//                d ← p2
	//
	circle, err := geom.CircleFromPoints(points[p1], points[ps], points[pe])
	if err != nil {
		return nil, err
	}

	if debug {
		log.Printf("Circle center point: \n%v\n", wkt.MustEncode(geom.Point(circle.Center)))
		log.Printf("Redius x: \n%v\n", wkt.MustEncode(
			geom.Line{
				geom.Point(circle.Center),
				geom.Point{circle.Center[0] + circle.Radius, circle.Center[1]},
			}))
		log.Printf("Circle Points: \n%v\n", wkt.MustEncode([]geom.Point{points[p1], points[ps], points[pe]}))
		log.Printf("Circle: \n%v\n", wkt.MustEncode(circle.AsPoints(36)))

		log.Printf("p2: %v, len(points):%v", p2, len(points))
	}

	if circle.ContainsPoint(points[p2]) {
		// we need to "flip" our edge from p1 to p2.
		//                a ← pe
		//               /|\
		//              / | \
		//             /  |  \
		//       ps → b   |   c ← p2
		//             \  |  /
		//              \ | /
		//               \|/
		//                d ← p1
		//
		if debug {
			log.Printf("p2 (%v) is contained. Flipping.", p2)
			log.Printf("From:\n%v", wkt.MustEncode([]geom.Line{
				geom.Line{points[ps], points[pe]},
				geom.Line{points[pe], points[p1]},
				geom.Line{points[p1], points[ps]},
			}))
			log.Printf("Proposed Shared edge %v", wkt.MustEncode(geom.Line{points[pe], points[p1]}))
		}
		ps, p1, p2, pe = pe, p2, p1, ps
		if debug {
			log.Printf("To:\n%v", wkt.MustEncode([]geom.Line{
				geom.Line{points[ps], points[pe]},
				geom.Line{points[pe], points[p1]},
				geom.Line{points[p1], points[ps]},
			}))
			log.Printf("Proposed Shared edge %v", wkt.MustEncode(geom.Line{points[pe], points[p1]}))
		}
	}

	// We need to check to see if the shared edge we have choosen is part of the external polygon.
	// if it is we move to the next edge of the triangle until we are on an edge that is not part of
	// the polygon. There should always be at least one edge that is not part of the polygon. Since,
	// the only time where all three edges are part of the polygon is if the polygon is an triangle,
	// and we have already check for that above.
	{
		secondCount := false
		count := 0
		if debug {
			log.Printf("Shared edge %v; rotating", wkt.MustEncode(geom.Line{points[pe], points[p1]}))
		}
		for em.Contains(points[pe], points[p1]) {
			pe, p1, ps = ps, pe, p1
			count++
			if count > 3 {
				for emLine, _ := range em {
					log.Printf("em:line: %v", wkt.MustEncode(emLine))
				}
				log.Printf("starting points(%v): %v", len(points), wkt.MustEncode(points))
				log.Printf("ps: %v p1: %v pe: %v -- p2: %v ", points[ps], points[p1], points[pe], points[p2])
				if secondCount {
					break
				}
				// flip p1 to p2
				p1, p2 = p2, p1
				count = 0
				secondCount = true

			}
			if debug {
				log.Printf("Shared edge %v; rotating", wkt.MustEncode(geom.Line{points[pe], points[p1]}))
			}
		}
		if secondCount {
			panic("assumption failed")
		}
	}

	if debug {
		log.Printf("ps: %v p1: %v p2: %v pe: %v", points[ps], points[p1], points[p2], points[pe])
		log.Printf("Shared edge %v", wkt.MustEncode(geom.Line{points[pe], points[p1]}))
	}

	// Let's do a quick check to see if there are only four points. If there are we have our edges.
	if plen == 4 {
		return []geom.Line{
			geom.Line{points[ps], points[p1]},
			geom.Line{points[pe], points[ps]},
			geom.Line{points[pe], points[p1]},
			// points[pe] to points[p1] is the shared edge of the two triangles.
			geom.Line{points[p1], points[p2]},
			geom.Line{points[p2], points[pe]},
		}, nil
	}

	ply := make([]geom.Point, 0, len(points))
	// We need to collect the point from pe -> p1 into a polygon
	var i = pe
	ply = append(ply, points[i])
	for {
		i++
		if i >= len(points) {
			i = 0
		}
		ply = append(ply, points[i])
		if i == p1 {
			break
		}
	}
	// We now need to triangulate the pseudo-polygon
	newEdges, err := triangulatePseudoPolygon(ply)
	if err != nil {
		log.Printf("Called Self(%v) with\n%v\n", len(ply), wkt.MustEncode(ply))
		return nil, err
	}
	edges = append(edges, newEdges...)

	ply = ply[:0]
	// and p1 -> pe into another polygon
	i = p1
	ply = append(ply, points[i])
	for {
		i++
		if i >= len(points) {
			i = 0
		}
		ply = append(ply, points[i])
		if i == pe {
			break
		}
	}
	// We now need to triangulate the pseudo-polygon
	newEdges, err = triangulatePseudoPolygon(ply)
	if err != nil {
		log.Printf("Called Self(%v) with\n%v\n", len(ply), wkt.MustEncode(ply))
		return nil, err
	}
	edges = append(edges, newEdges...)
	return edges, nil
}
