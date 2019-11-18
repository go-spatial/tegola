package intersect

import (
	"log"

	"github.com/go-spatial/geom"
	pkgcmp "github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar"
)

type Ring struct {
	segs          []geom.Line
	index         *SearchSegmentIdxs
	IncludeBorder bool

	CMP pkgcmp.Compare

	bbox geom.Extent
}

func NewRing(segs []geom.Line) *Ring {
	var index *SearchSegmentIdxs
	if len(segs) > len(staticIdxs) {
		// Only build index for large rings. Cost of building the index is
		// higher then the query efficiency for smaller rings.
		// TODO: re-evaluate cut-off if index implementation changes.
		index = NewSearchSegmentIdxs(segs)
	}
	r := &Ring{
		segs:  segs,
		index: index,
	}
	for i := range segs {
		r.bbox.AddPoints(segs[i][0], segs[i][1])
	}
	r.CMP = pkgcmp.DefaultCompare()
	return r
}

func NewRingFromPointers(pts ...geom.Pointer) *Ring {
	segs := make([]geom.Line, 0, len(pts))
	lp := len(pts) - 1
	for i := range pts {
		xy := pts[i].XY()
		lpxy := pts[lp].XY()
		segs = append(segs, geom.Line{lpxy, xy})
		lp = i
	}
	return NewRing(segs)
}

func NewRingFromPoints(pts ...[2]float64) *Ring {
	segs := make([]geom.Line, 0, len(pts))
	lp := len(pts) - 1
	for i := range pts {
		segs = append(segs, geom.Line{pts[lp], pts[i]})
		lp = i
	}
	return NewRing(segs)
}

func (r *Ring) Extent() *geom.Extent {
	if r == nil {
		return nil
	}
	return &r.bbox
}

// Static indices for Ring.ContainsPoint. We build our result slice from this
// array to avoid allocation of a new []int slice for each ContainsPoint call.
var staticIdxs = [...]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

func (r *Ring) ContainsPoint(pt [2]float64) bool {
	if r == nil {
		return false
	}

	cmp := r.CMP
	if debug {
		log.Printf("see of pt %+v is contained by ring: %+v ", pt, r.segs)
	}
	if !r.bbox.ContainsPoint(pt) {
		if debug {
			log.Printf("\t Point is not in intersect ring extent.")
		}
		return false
	}

	l := geom.Line{{r.bbox.MinX() - 1, pt[1]}, pt}

	var results []int
	if r.index != nil {
		results = r.index.SearchIntersectIdxs(l)
	} else {
		results = staticIdxs[:len(r.segs)]
	}

	if debug {
		log.Printf("\t SearchIntersect got back (%v):  %+v", len(results), results)
	}

	count, ok := 0, false
	var ipt [2]float64
	for _, idx := range results {
		if planar.AreLinesColinear(l, r.segs[idx]) {
			if debug {
				log.Printf("\t The lines are colinear.")
			}
			if r.segs[idx].ContainsPoint(pt) {
				// we are on the border, so return what include border tells us to return
				return r.IncludeBorder
			}
			continue
		}
		if ipt, ok = planar.SegmentIntersect(l, r.segs[idx]); !ok {
			if debug {
				log.Printf("\t The lines don't intersect %v: %v <=> %v", idx, r.segs[idx], l)
			}
			continue
		}
		if cmp.PointEqual(ipt, pt) {
			if debug {
				log.Printf("\t Intersect point is is the same as match point. %v returning %v", idx, r.IncludeBorder)
			}
			// we are on the border, so return what include border tells us to return
			return r.IncludeBorder
		}

		// check to see if ipt is on the end point of the segment, if so we will
		// only increment the counter for any lines below the current ray.
		if cmp.PointEqual(r.segs[idx][0], ipt) {
			if debug {
				log.Printf("\t Intersect point on the end point of the segment. %v", idx)
			}
			if r.segs[idx][1][1] > pt[1] {
				continue
			}
		} else if cmp.PointEqual(r.segs[idx][1], ipt) {
			if debug {
				log.Printf("\t Intersect point on the end point of the segment. %v", idx)
			}
			if r.segs[idx][0][1] > pt[1] {
				continue
			}
		}

		count++
	}
	if debug {
		log.Printf("\t count is %v", count)
	}
	// If it's even we are outside of the ring.
	return count%2 != 0
}
