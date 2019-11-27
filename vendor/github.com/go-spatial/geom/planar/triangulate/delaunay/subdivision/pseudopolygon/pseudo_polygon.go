package pseudopolygon

import (
	"log"
	"math"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/winding"
)

func triangulateSubRings(oPoints []geom.Point, order winding.Order) (points []geom.Point, edges []geom.Line, err error) {

	if debug {
		log.Printf("Step-1: starting points(%v): %v", len(oPoints), wkt.MustEncode(oPoints))
	}

	points = make([]geom.Point, 0, len(oPoints))

	// deal with duplicate points
	seenPts := make(map[geom.Point][2]int, len(oPoints))
	for i, pt := range oPoints {
		if idxs, seen := seenPts[pt]; seen {
			// let see if the duplicate point is the same as the last
			// point, in which, just drop that dup and move on.
			if idxs[0] == i-1 {
				seenPts[pt] = [2]int{i, idxs[1]}
				continue
			}

			npts := make([]geom.Point, i-idxs[0])
			copy(npts, oPoints[idxs[0]:i])
			points = points[:idxs[1]+1]
			// We have seen this point before.
			// What we need to do, is split out the points and triangulate them
			if debug {
				log.Printf("dup point at %v : points %v",
					i,
					wkt.MustEncode(npts),
				)
			}
			// Not sure which is the correct way to deal with single lines here.
			// Weather we should be dropping them or including them.
			// seems like for what we are using this for including them makes sense
			// , but logically we should drop them. To drop them, uncomment the guard
			// and modify test `multiple duplicated points`
			//if len(npts) > 2 {
			newEdges, err := Triangulate(npts, order)
			if err != nil {
				return nil, nil, err
			}
			edges = append(edges, newEdges...)
			//}
			// Clean up map
			for _, pt := range npts {
				delete(seenPts, pt)
			}
			seenPts[pt] = [2]int{i, len(points) - 1}
			continue
		}
		seenPts[pt] = [2]int{i, len(points)}
		points = append(points, pt)
	}
	if debug {
		log.Printf("Step 0: starting points(%v): %v", len(points), wkt.MustEncode(points))
	}
	points = points[0:len(points):len(points)]
	return points, edges, nil

}

// Triangulate will return triangulated edges for the given polygon. The edges are not
// guaranteed to be unique or normalized.
func Triangulate(oPoints []geom.Point, order winding.Order) (edges []geom.Line, err error) {

	if debug {
		log.Printf("opoints:\n%v", wkt.MustEncode(oPoints))
	}
	var points []geom.Point

	points, edges, err = triangulateSubRings(oPoints, order)
	if err != nil {
		return nil, err
	}
	if debug && len(edges) > 0 {
		log.Printf("got Edges from triangulateSubRings: %v", wkt.MustEncode(edges))
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

	if order.OfGeomPoints(points...).IsColinear() {
		if debug {
			log.Printf("Step 0: colinear starting points(%v): %v", len(points), wkt.MustEncode(points))
		}
		return nil, ErrAllPointsColinear
	}

	if plen == 3 {
		edges = append(edges,
			geom.Line{points[0], points[1]},
			geom.Line{points[1], points[2]},
			geom.Line{points[2], points[0]},
		)
		if debug {
			log.Printf("returning Edges: %v", wkt.MustEncode(edges))
		}
		return edges, nil
	}

	em := newEdgeMap(points)

	// find the next two closes points to the center of the line between the
	// first and last points.
	x1, y1, x2, y2 := points[0][0], points[0][1], points[plen-1][0], points[plen-1][1]

	cpoint := geom.Point{(x1 + x2) / 2, (y1 + y2) / 2}
	if debug {
		log.Printf("center point: %v", wkt.MustEncode(cpoint))
	}
	dist := math.Inf(1)
	var ps, p1, p2, pe int = 0, -1, -1, plen - 1

	for i, candidate := range points[1:pe] {
		d := planar.PointDistance(cpoint, candidate)
		cln := order.OfGeomPoints(points[ps], points[i+1], points[pe])
		if debug {
			log.Printf("colin: %v -- %v %v %v", cln, points[ps], points[i+1], points[pe])
			log.Printf("%v distance: %v < %v : %v / %v", i+1, d, dist, d < dist, cln)
			log.Printf("p1: %v p2: %v dist: %v", p1, p2, dist)
		}
		if d < dist && !cln.IsColinear() {
			p1, p2, dist = i+1, p1, d
		}
	}

	if debug {
		log.Printf("Step 1: Line to point: %v", wkt.MustEncode(geom.Line{points[0], points[plen-1]}))
		if p1 != -1 {
			log.Printf("Step 1: Found closes point: %v ", wkt.MustEncode(points[p1]))
		}
		log.Printf("ps: %v p1: %v p2: %v pe: %v", ps, p1, p2, pe)
	}

	if p2 == -1 {
		// try the previous point of p1
		p2 = p1 - 1
		if p2 == ps || p2 == pe {
			p2 = p1 + 1
		}
	}

	if debug {
		log.Printf("Step 1a: p2: %v -- %v", p2, len(points))
		log.Printf("Step 1a: Found next closes point: %v ", wkt.MustEncode(points[p2]))
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

	// Should never really error, as the only error is if the points are colinear, and we checked that already
	circle, _ := geom.CircleFromPoints(points[p1], points[ps], points[pe])

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

	p2IsCol := order.OfGeomPoints(points[ps], points[p2], points[pe]).IsColinear()
	if !p2IsCol && circle.ContainsPoint(points[p2]) {
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

	// We need to check to see if the shared edge we have chosen is part of the external polygon.
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
		edges = append(edges,
			geom.Line{points[ps], points[p1]},
			geom.Line{points[pe], points[ps]},
			geom.Line{points[pe], points[p1]},
			// points[pe] to points[p1] is the shared edge of the two triangles.
			geom.Line{points[p1], points[p2]},
			geom.Line{points[p2], points[pe]},
		)
		if debug {
			log.Printf("returning Edges: %v", wkt.MustEncode(edges))
		}
		return edges, nil
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
	newEdges, err := Triangulate(ply, order)
	if err != nil {
		log.Printf("Called Self(%v) with\n%v\n", len(ply), wkt.MustEncode(ply))
		return nil, err
	}
	edges = append(edges, newEdges...)
	if debug {
		log.Printf("got Edges: %v", wkt.MustEncode(edges))
	}

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
	newEdges, err = Triangulate(ply, order)
	if err != nil {
		if debug {
			log.Printf("Called Self(%v) with\n%v\n", len(ply), wkt.MustEncode(ply))
		}
		return nil, err
	}
	edges = append(edges, newEdges...)
	if debug {
		log.Printf("returning Edges: %v", wkt.MustEncode(edges))
	}
	return edges, nil
}
