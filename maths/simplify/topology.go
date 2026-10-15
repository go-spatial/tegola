package simplify

import (
	"math"
	"sort"

	"github.com/alexeydott/tegola/maths"
)

// Segment-level geometry predicates that make simplification intersection
// aware (UPSTREAM.md debt 3.6). All predicates use a relative epsilon so they
// behave consistently across coordinate ranges from projected degrees (~1) to
// web-mercator meters (~2e7).
//
// Self-intersection semantics (mirrored by the correctness corpus in
// basic/line_simplify_corpus_test.go):
//
//   - non-adjacent segments intersecting in any point (proper crossing,
//     endpoint touch or collinear overlap) is a self-intersection;
//   - adjacent segments are allowed to meet at their shared vertex and to run
//     collinearly past it (a monotone straight run), but not to fold back over
//     each other;
//   - for rings the closing segment is adjacent to the first and last chain
//     segments and the same rules apply.

const relEps = 1e-12

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// nearZero reports whether v is zero within a scale-relative epsilon.
func nearZero(v, scale float64) bool {
	return math.Abs(v) <= relEps*maxFloat(1, scale)
}

func sub2(a, b maths.Pt) maths.Pt { return maths.Pt{X: a.X - b.X, Y: a.Y - b.Y} }

func cross2(u, v maths.Pt) float64 { return u.X*v.Y - u.Y*v.X }

func dot2(u, v maths.Pt) float64 { return u.X*v.X + u.Y*v.Y }

// segDistPoint returns the distance from p to the finite segment a-b (the
// distance to the closest endpoint when a == b). Unlike
// maths.Line.DistanceFromPoint, which measures against the infinite line
// through a and b, points past either end of the segment count at their true
// distance — collapsing them would fold the line back over itself ("needle"
// artifacts).
func segDistPoint(p, a, b maths.Pt) float64 {
	r := sub2(b, a)
	rr := dot2(r, r)
	if rr == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := dot2(sub2(p, a), r) / rr
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(p.X-(a.X+t*r.X), p.Y-(a.Y+t*r.Y))
}

// bboxDisjoint reports whether segments a-b and c-d have disjoint bounding
// boxes and therefore cannot intersect.
func bboxDisjoint(a, b, c, d maths.Pt) bool {
	return math.Min(a.X, b.X) > math.Max(c.X, d.X) ||
		math.Min(c.X, d.X) > math.Max(a.X, b.X) ||
		math.Min(a.Y, b.Y) > math.Max(c.Y, d.Y) ||
		math.Min(c.Y, d.Y) > math.Max(a.Y, b.Y)
}

// segProperCross reports whether segments a-b and c-d cross transversally,
// with the crossing point strictly interior to both segments. Endpoint
// touches and collinear overlaps are not proper crossings: a replacement
// chord may legitimately overlap the sub-polyline it replaces.
func segProperCross(a, b, c, d maths.Pt) bool {
	r := sub2(b, a)
	s := sub2(d, c)
	den := cross2(r, s)
	if nearZero(den, math.Sqrt(dot2(r, r)*dot2(s, s))) {
		return false // parallel, collinear or degenerate
	}
	ca := sub2(c, a)
	t := cross2(ca, s) / den
	u := cross2(ca, r) / den
	return t > relEps && t < 1-relEps && u > relEps && u < 1-relEps
}

// projOverlap reports whether collinear segments a-b and c-d overlap in
// space (touching at an endpoint counts). Collinear segments overlap exactly
// when their bounding boxes overlap on both axes.
func projOverlap(a, b, c, d maths.Pt) bool {
	return math.Min(a.X, b.X) <= math.Max(c.X, d.X) &&
		math.Min(c.X, d.X) <= math.Max(a.X, b.X) &&
		math.Min(a.Y, b.Y) <= math.Max(c.Y, d.Y) &&
		math.Min(c.Y, d.Y) <= math.Max(a.Y, b.Y)
}

// segSharePoint reports whether segments a-b and c-d share at least one
// point: a proper crossing, an endpoint touch or a collinear overlap.
func segSharePoint(a, b, c, d maths.Pt) bool {
	r := sub2(b, a)
	s := sub2(d, c)
	den := cross2(r, s)
	ca := sub2(c, a)
	if !nearZero(den, math.Sqrt(dot2(r, r)*dot2(s, s))) {
		t := cross2(ca, s) / den
		u := cross2(ca, r) / den
		return t >= -relEps && t <= 1+relEps && u >= -relEps && u <= 1+relEps
	}
	// Parallel: share a point only if collinear and their extents overlap.
	if !nearZero(cross2(ca, r), math.Sqrt(dot2(ca, ca)*dot2(r, r))) {
		return false
	}
	return projOverlap(a, b, c, d)
}

// foldBack reports whether the consecutive segments a-b and b-d overlap
// beyond their shared vertex b: they are collinear and both unshared ends
// lie on the same side of b, so the path doubles back over itself.
func foldBack(a, b, d maths.Pt) bool {
	ba := sub2(a, b)
	bd := sub2(d, b)
	if !nearZero(cross2(ba, bd), math.Sqrt(dot2(ba, ba)*dot2(bd, bd))) {
		return false // not collinear
	}
	return dot2(ba, bd) > 0
}

// ceilLog2 returns ceil(log2(n)) for n >= 1 (0 for n <= 1).
func ceilLog2(n int) int {
	log := 0
	for (1 << log) < n {
		log++
	}
	return log
}

// pairBudget returns the maximum number of segment-pair examinations a single
// self-intersection scan may spend. Keeps the scans near O(n log n); when the
// budget runs out the scan reports "unknown" and callers fall back to the
// unsimplified input (correctness over compression).
func pairBudget(segCount int) int {
	return 4*segCount*ceilLog2(segCount) + 4096
}

// segBox is a segment bounding box tagged with its segment index.
type segBox struct {
	minX, maxX, minY, maxY float64
	i                      int
}

// selfIntersects reports whether the polyline pts intersects itself per the
// rules at the top of this file. When ring is true the closing segment
// pts[len-1]-pts[0] is included and treated as adjacent to the first and last
// chain segments.
//
// complete is false when the pair budget was exhausted before a decision; the
// caller must treat that as "unknown" and prefer the safe (unsimplified)
// outcome. Segments are examined in x-sorted order with a bounding-box
// pre-filter so ordinary (sparse) geometries cost near-linear work.
func selfIntersects(pts []maths.Pt, ring bool) (intersects, complete bool) {
	n := len(pts)
	if n < 2 {
		return false, true
	}
	segCount := n - 1
	if ring {
		segCount = n
	}
	boxes := make([]segBox, segCount)
	for i := 0; i < segCount; i++ {
		a, b := pts[i], pts[(i+1)%n]
		boxes[i] = segBox{
			minX: math.Min(a.X, b.X), maxX: math.Max(a.X, b.X),
			minY: math.Min(a.Y, b.Y), maxY: math.Max(a.Y, b.Y),
			i: i,
		}
	}
	sort.Slice(boxes, func(x, y int) bool { return boxes[x].minX < boxes[y].minX })

	budget := pairBudget(segCount)
	for s := 0; s < segCount; s++ {
		for t := s + 1; t < segCount && boxes[t].minX <= boxes[s].maxX; t++ {
			budget--
			if budget < 0 {
				return false, false
			}
			if boxes[t].minY > boxes[s].maxY || boxes[s].minY > boxes[t].maxY {
				continue
			}
			i, j := boxes[s].i, boxes[t].i
			if i > j {
				i, j = j, i
			}
			a, b := pts[i], pts[(i+1)%n]
			c, d := pts[j], pts[(j+1)%n]
			switch {
			case j == i+1:
				if foldBack(a, b, d) {
					return true, true
				}
			case ring && i == 0 && j == segCount-1:
				// closing segment pts[n-1]-pts[0] adjacent to pts[0]-pts[1]
				if foldBack(pts[n-1], pts[0], pts[1]) {
					return true, true
				}
			default:
				if segSharePoint(a, b, c, d) {
					return true, true
				}
			}
		}
	}
	return false, true
}

// chordCrossesOutside reports whether the candidate chord joining all[lo] to
// all[hi] properly crosses any segment of the full polyline that lies outside
// the span it replaces. Crossings with the span's own sub-polyline are
// allowed (those points are removed, and the two-sided tolerance bound still
// holds); crossing geometry that remains in the result is what turns a
// collapse into a self-intersection.
//
// The scan is linear in the polyline length per candidate with a bounding-box
// pre-filter; every iteration draws down the shared pair budget, so the total
// work across a DouglasPeucker run stays near O(n log n). When the budget is
// exhausted the check reports false (no violation) and the final whole-line
// validation in DouglasPeucker backstops correctness.
func chordCrossesOutside(all []maths.Pt, lo, hi int, budget *int) bool {
	if hi <= lo+1 {
		return false // nothing is replaced: the chord is original geometry
	}
	a, b := all[lo], all[hi]
	for k := 0; k < len(all)-1; k++ {
		if k >= lo && k < hi {
			continue // replaced sub-polyline
		}
		*budget--
		if *budget < 0 {
			return false
		}
		c, d := all[k], all[k+1]
		if bboxDisjoint(a, b, c, d) {
			continue
		}
		if segProperCross(a, b, c, d) {
			return true
		}
	}
	return false
}
