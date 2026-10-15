package simplify

import (
	"math"
	"sort"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/maths"
)

func openRing(pts []maths.Pt) []maths.Pt {
	if len(pts) > 1 && pts[0] == pts[len(pts)-1] {
		return pts[:len(pts)-1]
	}
	return pts
}

// Translate to the first vertex before accumulating area to avoid cancellation
// for small polygons at large projected coordinate offsets.
func ringArea(pts []maths.Pt) float64 {
	if len(pts) < 3 {
		return 0
	}
	var area float64
	for i := 1; i+1 < len(pts); i++ {
		area += cross2(sub2(pts[i], pts[0]), sub2(pts[i+1], pts[0]))
	}
	return area / 2
}

func validRing(pts []maths.Pt) bool {
	if len(pts) < 3 {
		return false
	}
	for i, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || p == pts[(i+1)%len(pts)] {
			return false
		}
	}
	a := ringArea(pts)
	if a == 0 || math.IsNaN(a) || math.IsInf(a, 0) {
		return false
	}
	intersects, complete := selfIntersects(pts, true)
	return complete && !intersects
}

// pointInRing is used only after proving that two simple ring boundaries do
// not meet, so one representative vertex determines their containment.
func pointInRing(p maths.Pt, ring []maths.Pt, budget *int) (bool, bool) {
	inside := false
	for i, a := range ring {
		*budget--
		if *budget < 0 {
			return false, false
		}
		b := ring[(i+1)%len(ring)]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < a.X+(p.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y) {
			inside = !inside
		}
	}
	return inside, true
}

// Returns 0 for disjoint rings, 1 for a inside b, 2 for b inside a.
// Touches (including existing touches), crossings and an exhausted work budget
// are deliberately inconclusive: preserve the complete original geometry.
func ringRelationship(a, b []maths.Pt, budget *int) (int, bool) {
	boxes := make([]segBox, 0, len(a)+len(b))
	for group, ring := range [][]maths.Pt{a, b} {
		for i, p := range ring {
			q := ring[(i+1)%len(ring)]
			index := i
			if group == 1 {
				index += len(a)
			}
			boxes = append(boxes, segBox{math.Min(p.X, q.X), math.Max(p.X, q.X), math.Min(p.Y, q.Y), math.Max(p.Y, q.Y), index})
		}
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].minX < boxes[j].minX })
	for i, p := range boxes {
		for j := i + 1; j < len(boxes) && boxes[j].minX <= p.maxX; j++ {
			*budget--
			if *budget < 0 {
				return 0, false
			}
			q := boxes[j]
			if (p.i < len(a)) == (q.i < len(a)) || p.minY > q.maxY || q.minY > p.maxY {
				continue
			}
			x, y := p.i, q.i
			if x >= len(a) {
				x, y = y, x
			}
			y -= len(a)
			if segSharePoint(a[x], a[(x+1)%len(a)], b[y], b[(y+1)%len(b)]) {
				return 0, false
			}
		}
	}
	inside, complete := pointInRing(a[0], b, budget)
	if !complete || inside {
		return 1, complete
	}
	inside, complete = pointInRing(b[0], a, budget)
	if inside {
		return 2, complete
	}
	return 0, complete
}

type topologyRing struct {
	before, after []maths.Pt
	polygon, ring int
}

func preservesPolygonTopology(original, candidate basic.MultiPolygon) bool {
	var rings []topologyRing
	vertices := 0
	for i, polygon := range original {
		if len(polygon) == 0 || len(candidate[i]) != len(polygon) {
			return false
		}
		for j, line := range polygon {
			a, b := openRing(line.AsPts()), openRing(candidate[i][j].AsPts())
			if !validRing(a) || !validRing(b) || ringArea(a)*ringArea(b) <= 0 {
				return false
			}
			rings = append(rings, topologyRing{a, b, i, j})
			vertices += len(a) + len(b)
		}
	}
	budget := pairBudget(vertices)
	for i, a := range rings {
		for _, b := range rings[i+1:] {
			before, complete := ringRelationship(a.before, b.before, &budget)
			if !complete {
				return false
			}
			if a.polygon == b.polygon {
				// Every hole lies strictly inside its shell; holes are disjoint.
				if (a.ring == 0 && before != 2) || (a.ring != 0 && before != 0) {
					return false
				}
			}
			after, complete := ringRelationship(a.after, b.after, &budget)
			if !complete || before != after {
				return false
			}
		}
	}
	// A component may be inside another component's hole (an island), but not
	// its filled interior. Boundary disjointness was established above.
	for i, polygon := range original {
		p := polygon[0].AsPts()[0]
		for j, other := range original {
			if i == j {
				continue
			}
			filled := false
			for k, line := range other {
				inside, complete := pointInRing(p, openRing(line.AsPts()), &budget)
				if !complete {
					return false
				}
				if k == 0 {
					filled = inside
				} else if inside {
					filled = false
				}
			}
			if filled {
				return false
			}
		}
	}
	return true
}
