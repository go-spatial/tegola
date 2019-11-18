package walker

import (
	"context"
	"log"

	"github.com/go-spatial/geom/winding"

	"github.com/go-spatial/geom/encoding/wkt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar"
)

func sortedEdge(pt1, pt2 [2]float64) [2][2]float64 {
	if cmp.PointLess(pt1, pt2) {
		return [2][2]float64{pt1, pt2}
	}
	return [2][2]float64{pt2, pt1}
}

func edgeMapFromTriangles(triangles ...geom.Triangle) map[[2][2]float64][]int {
	// an edge can have at most two triangles associated with it.
	em := make(map[[2][2]float64][]int, 2*len(triangles))
	for i, tri := range triangles {
		for _, edg := range SortedEdges(tri) {
			if _, ok := em[edg]; !ok {
				em[edg] = make([]int, 0, 2)
			}
			em[edg] = append(em[edg], i)
		}
	}
	return em
}

func SortedEdges(t geom.Triangle) [3][2][2]float64 {
	return [3][2][2]float64{
		sortedEdge(t[0], t[1]),
		sortedEdge(t[0], t[2]),
		sortedEdge(t[1], t[2]),
	}
}

func MultiPolygon(ctx context.Context, triangles []geom.Triangle) geom.MultiPolygon {
	return New(triangles).MultiPolygon(ctx)
}

// New creates a new walker that can be used to fix a geometry.
func New(triangles []geom.Triangle) *Walker {

	return &Walker{
		Triangles: triangles,
		edgeMap:   edgeMapFromTriangles(triangles...),
	}
}

type Walker struct {
	Triangles []geom.Triangle
	edgeMap   map[[2][2]float64][]int
	Order     winding.Order
}

// EdgeMap returns a copy of the edgemap
func (w *Walker) EdgeMap() (edgeMap map[[2][2]float64][]int) {
	if w == nil {
		return edgeMap
	}
	edgeMap = make(map[[2][2]float64][]int, len(w.edgeMap))
	for k, v := range w.edgeMap {
		edgeMap[k] = v
	}
	return edgeMap
}

// MultiPolygon walks all triangles and returns the generated polygons as a multipolygon.
func (w *Walker) MultiPolygon(ctx context.Context) (mplyg geom.MultiPolygon) {
	if w == nil {
		if debug {
			log.Printf("walker is nil.")
		}
		return mplyg
	}
	if w.edgeMap == nil {
		w.edgeMap = edgeMapFromTriangles(w.Triangles...)
	}
	seen := make(map[int]bool, len(w.Triangles))
	for i := range w.Triangles {
		if ctx.Err() != nil {
			return nil
		}
		if seen[i] {
			continue
		}
		seen[i] = true
		plyg := w.PolygonForTriangle(ctx, i, seen)
		if debug {
			log.Printf(" %v : got the following plyg\n%v\n", i, wkt.MustEncode(geom.Polygon(plyg)))
		}
		if len(plyg) > 0 {
			mplyg = append(mplyg, plyg)
		}
	}
	return geom.MultiPolygon(mplyg)
}

// PolygonForTriangle walks the triangles starting at the given triangle returning the generated polygon from the walk.
func (w *Walker) PolygonForTriangle(ctx context.Context, idx int, seen map[int]bool) (plyg [][][2]float64) {
	// Get the external ring for the given triangle.
	plyg4r := PolygonForRing(ctx, w.RingForTriangle(ctx, idx, seen))
	return w.Order.RectifyPolygon(plyg4r)
}

// RingForTriangle will walk the set of triangles starting at the given triangle index. As it walks the triangles it will
// mark them as seen on the seen map. The function will return the outside ring of the walk
// if seen is nil, the function will panic.
func (w *Walker) RingForTriangle(ctx context.Context, idx int, seen map[int]bool) (rng [][2]float64) {
	if w == nil {
		return rng
	}

	var ok bool

	if debug {
		log.Printf("getting ring for triangle %v", idx)
	}

	seen[idx] = true

	// This tracks the start of the ring.
	// The segment we are adding a point will be between the endpoint and the beginning of the ring.
	// This tracks the original beginning of the ring.
	var headIdx int

	rng = append(rng, w.Triangles[idx][:]...)
	cidxs := []int{idx, idx, idx}
	cidx := cidxs[len(cidxs)-1]

RING_LOOP:
	for {
		// A few sanity checks, were we cancelled, or reached the end of our walk.
		if ctx.Err() != nil || // We were told to come home.
			headIdx >= len(rng) || len(cidxs) == 0 { // we returned home.
			return rng
		}

		if debug {
			log.Printf("headIdx: %v -- len(rng): %v", headIdx, len(rng))
			log.Printf("ring: %v | %v", rng[:headIdx], rng[headIdx:])
			log.Printf("cidxs: %v", cidxs)
		}

		if cidx, ok = w.indexForEdge(rng[0], rng[len(rng)-1], cidxs[len(cidxs)-1], seen); !ok {
			// We don't have a neighbor to walk to here. Let's move back one and see if there is a path we need to go down.
			headIdx += 1
			lpt := rng[len(rng)-1]
			copy(rng[1:], rng)
			rng[0] = lpt
			cidxs = cidxs[:len(cidxs)-1]
			continue
		}

		if cidx == idx {
			// We go back to our starting triangle. We need to stop.
			return rng
		}

		if debug {
			log.Printf("check to see if we have seen the triangle we are going to jump to.")
		}

		// Check to see if we have reached the triangle before.
		for i, pcidx := range cidxs {
			if pcidx != cidx {
				continue
			}
			if debug {
				log.Printf("we have encountered idx (%v) before at %v", cidx, i)
			}
			// need to move all the points over
			tlen := len(rng) - (i + 1)
			tpts := make([][2]float64, tlen)
			copy(tpts, rng[i+1:])
			copy(rng[tlen:], rng[:i+1])
			copy(rng, tpts)
			headIdx += tlen

			cidxs = cidxs[:i+1]
			continue RING_LOOP
		}

		rng = append(rng, w.Triangles[cidx].ThirdPoint(rng[0], rng[len(rng)-1]))

		cidxs[len(cidxs)-1] = cidx
		cidxs = append(cidxs, cidx)
		seen[cidx] = true

	} // for loop
	return rng
}

func (w *Walker) indexForEdge(p1, p2 [2]float64, defaultIdx int, seen map[int]bool) (idx int, ok bool) {
	for _, idx := range w.edgeMap[sortedEdge(p1, p2)] {
		if seen[idx] || idx == defaultIdx {
			continue
		}
		return idx, true
	}
	return defaultIdx, false
}

// PolygonForRing returns a polygon for the given ring, this will destroy the ring.
func PolygonForRing(ctx context.Context, rng [][2]float64) (plyg [][][2]float64) {
	if debug {
		log.Printf("turn ring into polygon.")
		log.Printf("ring: %v", wkt.MustEncode(rng))
	}

	if len(rng) <= 2 {
		return nil
	}

	// normalize ring
	cmp.RotateToLeftMostPoint(rng)

	pIdx := func(i int) int {
		if i == 0 {
			return len(rng) - 1
		}
		return i - 1
	}
	nIdx := func(i int) int {
		if i == len(rng)-1 {
			return 0
		}
		return i + 1
	}

	// Allocate space for the initial ring.
	plyg = make([][][2]float64, 1)

	// Remove bubbles. There are two types of bubbles we have to look for.
	// 1. ab … bc, in which case we need to hold on to b.
	//    It is possible that b is absolutely not necessary. It could lie on the line between a and c, in which case
	//    we should remove the extra point.
	// 2. ab … ba, which case we do not need to have b in the ring.

	// let's build an index of where the points that we are walking are. That way when we encounter the same
	// point we are able to “jump” to that point.
	ptIndex := map[[2]float64]int{}
	var ok bool
	var idx int

	// Let's walk the points
	for i := 0; i < len(rng); i++ {
		// Context has been cancelled.
		if ctx.Err() != nil {
			return nil
		}

		// check to see if we have already seen this point.
		if idx, ok = ptIndex[rng[i]]; !ok {
			ptIndex[rng[i]] = i
			continue
		}

		// ➠ what type of bubble are we dealing with
		pidx, nidx := pIdx(idx), nIdx(i)

		// clear out ptIndex of the values we are going to cut.
		for j := idx; j <= i; j++ {
			delete(ptIndex, rng[j])
		}

		// ab…ba ring. So we need to remove all the way to a.
		if nidx != pidx && cmp.PointEqual(rng[pidx], rng[nidx]) {
			if debug {
				log.Printf("bubble type ab…ba: (% 5v)(% 5v) … (% 5v)(% 5v)", pidx, idx, i, nidx)
			}

			// delete the ʽaʼ point as well from point index
			delete(ptIndex, rng[pidx])

			sliver := cut(&rng, pidx, nidx)
			// remove the start ab
			sliver = sliver[2:]
			if len(sliver) >= 3 { // make a copy to free up memory.
				ps := make([][2]float64, len(sliver))
				copy(ps, sliver)
				cmp.RotateToLeftMostPoint(ps)
				if debug {
					log.Printf("ring: %v", wkt.MustEncode(rng))
					log.Printf("sliver: %v", wkt.MustEncode(sliver))
				}
				plyg = append(plyg, ps)
			}

			if i = idx - 1; i < 0 {
				i = 0
			}
			continue
		}

		// do a quick check to see if b is on ac
		removeB := planar.IsPointOnLine(rng[i], rng[pidx], rng[nidx])

		// ab … bc The sliver is going to be from b … just before b. So, the ring will be …abc… or …ac… depending on removeB
		if debug {
			log.Printf("bubble type ab…bc: (% 5v)(% 5v) … (% 5v)(% 5v) == %t", pidx, idx, i, nidx, removeB)
			log.Printf("ab..bc: [%v][%v]..[%v][%v]",
				wkt.MustEncode(geom.Point(rng[pidx])),
				wkt.MustEncode(geom.Point(rng[idx])),
				wkt.MustEncode(geom.Point(rng[i])),
				wkt.MustEncode(geom.Point(rng[nidx])),
			)
			log.Printf("%v , %v",
				wkt.MustEncode(geom.Point(rng[i])),
				wkt.MustEncode(geom.Line{rng[pidx], rng[nidx]}),
			)
		}

		// Quick hack to remove extra bridges that are geting left over.
		sliver := removeBridge(cut(&rng, idx, i))

		if len(sliver) >= 3 {
			cmp.RotateToLeftMostPoint(sliver)
			if debug {
				log.Printf("ring: %v", wkt.MustEncode(rng))
				log.Printf("sliver: %v", wkt.MustEncode(sliver))
			}
			plyg = append(plyg, sliver)
		}

		i = idx
		if removeB {
			cut(&rng, idx, idx+1)
			if idx == 0 {
				break
			}
			i = idx - 1
		}
	} // for

	if len(rng) <= 2 {
		if debug {
			log.Println("rng:", rng)
			log.Println("plyg:", plyg)
			panic("main ring is not correct!")
		}
		return nil
	}

	plyg[0] = make([][2]float64, len(rng))
	copy(plyg[0], rng)
	cmp.RotateToLeftMostPoint(plyg[0])
	return plyg
}
