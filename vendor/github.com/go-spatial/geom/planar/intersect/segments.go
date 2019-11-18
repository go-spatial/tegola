package intersect

import (
	"log"
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/internal/rtreego"
)

const smallep = 0.00001

type segRect struct {
	idx  int
	rect *rtreego.Rect
}

func (sr *segRect) Bounds() *rtreego.Rect { return sr.rect }

func minAndDelta(c1, c2 float64) (min, delta float64) {
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	return c1, (c2 - c1) + smallep
}

func rectForSegment(seg geom.Line) (rect *rtreego.Rect) {
	var err error
	minx, deltax := minAndDelta(seg[0][0], seg[1][0])
	miny, deltay := minAndDelta(seg[0][1], seg[1][1])
	// We are insuring that an error can not happen.
	if rect, err = rtreego.NewRect(rtreego.Point{minx, miny}, []float64{deltax, deltay}); err != nil {
		panic("Assumption broken:" + err.Error())
	}
	return rect
}

type SegmentFilterFn func(result []int, seg geom.Line, idx int) (refuse, abort bool)

type SearchSegmentIdxs struct {
	tree *rtreego.Rtree
}

func (segs *SearchSegmentIdxs) SearchIntersectIdxs(seg geom.Line, filters ...SegmentFilterFn) (idxs []int) {
	if segs == nil || segs.tree == nil {
		return nil
	}
	if debug {
		log.Printf("Looking for segment %v", seg)
	}
	results := segs.tree.SearchIntersect(rectForSegment(seg))
	var rseg *segRect
	for _, r := range results {
		var abort, refuse, ok bool
		if rseg, ok = r.(*segRect); !ok {
			continue
		}
		for _, filterfn := range filters {
			var abortf bool
			refuse, abortf = filterfn(idxs, seg, rseg.idx)
			if refuse {
				break
			}
			abort = abort || abortf
		}
		if !refuse {
			idxs = append(idxs, rseg.idx)
		}
		if abort {
			return idxs
		}
	}
	sort.Ints(idxs)
	return idxs
}

func NewSearchSegmentIdxs(segs []geom.Line) *SearchSegmentIdxs {
	if len(segs) <= 1 {
		return nil
	}

	segsRects := make([]rtreego.Spatial, len(segs))

	for i := range segs {
		segsRects[i] = &segRect{
			idx:  i,
			rect: rectForSegment(segs[i]),
		}
	}
	return &SearchSegmentIdxs{
		tree: rtreego.NewTree(1, 2, 5, segsRects...),
	}
}
