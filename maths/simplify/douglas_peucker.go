package simplify

import (
	"github.com/alexeydott/tegola/maths"
)

// DouglasPeucker is a geometry simplification routine
// https://en.wikipedia.org/wiki/Ramer%E2%80%93Douglas%E2%80%93Peucker_algorithm
//
// tolerance is a distance in the same units as the point coordinates.
// Callers that hold a squared tolerance must take its square root before
// calling; this function uses the tolerance as-is and passes it unchanged
// through the recursion (previously it squared the tolerance on entry and
// passed the squared value back into recursive calls, so the effective
// threshold collapsed to tolerance⁴/⁸/… and near-degenerate geometries were
// "simplified" into self-intersecting rings).
//
// The classic routine drops points on chord distance alone, which lets a
// replacement chord fold the line back over itself ("needle" artifacts) or
// cross geometry the chord does not replace, producing self-intersecting
// output (UPSTREAM.md debt 3.6). This implementation is intersection aware:
//
//   - distances are measured against the finite chord segment (not the
//     infinite line through it), so spikes past a chord end are kept;
//   - a candidate chord may not properly cross polyline segments outside the
//     span it replaces (bounding-box pre-filtered, budgeted across the run);
//   - a chord that violates that rule keeps the farthest point of its span
//     and both halves recurse (topology-preserving refinement);
//   - the whole result is validated for self-intersection whenever the input
//     was simple and falls back to the unsimplified input otherwise;
//   - deep recursion keeps its span verbatim instead of collapsing it.
//
// The result is always an ordered subsequence of the input and every removed
// vertex lies within tolerance of the replacement chord, so the two-sided
// Hausdorff distance between input and output is at most tolerance.
func DouglasPeucker(points []maths.Pt, tolerance float64) []maths.Pt {
	if tolerance <= 0 || len(points) <= 2 {
		return points
	}

	budget := pairBudget(len(points) - 1)
	out := dpSimplify(points, 0, len(points)-1, tolerance, 2*ceilLog2(len(points))+16, &budget)
	if len(out) == 0 {
		return points
	}

	// Whole-line topology backstop: chords that individually pass validation
	// can still combine into a self-intersection. When in doubt keep the
	// input; correctness over compression.
	if len(out) < len(points) {
		if intersects, complete := selfIntersects(out, false); !complete || intersects {
			if inIntersects, inComplete := selfIntersects(points, false); !inComplete || !inIntersects {
				return points
			}
		}
	}
	return out
}

// dpSimplify simplifies all[lo:hi+1] and returns the kept points. Absolute
// indices into all are used throughout because chordCrossesOutside must see
// the geometry outside the span being collapsed.
func dpSimplify(all []maths.Pt, lo, hi int, tolerance float64, depth int, budget *int) []maths.Pt {
	if hi <= lo+1 {
		return all[lo : hi+1]
	}
	if depth <= 0 {
		// Safety valve: keep the span verbatim instead of collapsing it.
		return all[lo : hi+1]
	}

	// find the farthest interior point from the chord segment a-b.
	// NOTE: the loop must cover every intermediate point; a len-2 skip
	// silently collapsed any 3-point slice to its endpoints no matter how far
	// the middle point sat from the chord ("bowtie" rings).
	a, b := all[lo], all[hi]
	dmax := 0.0
	idx := 0
	for i := lo + 1; i < hi; i++ {
		if d := segDistPoint(all[i], a, b); d > dmax {
			dmax = d
			idx = i
		}
	}

	if dmax > tolerance || chordCrossesOutside(all, lo, hi, budget) {
		if idx == 0 {
			idx = lo + 1 // degenerate chord: split at the first interior point
		}
		rec1 := dpSimplify(all, lo, idx, tolerance, depth-1, budget)
		rec2 := dpSimplify(all, idx, hi, tolerance, depth-1, budget)

		// The split point belongs to both recursive ranges; retain it only
		// once when joining the simplified segments.
		newpts := make([]maths.Pt, 0, len(rec1)+len(rec2)-1)
		newpts = append(newpts, rec1[:len(rec1)-1]...)
		newpts = append(newpts, rec2...)

		return newpts
	}

	return []maths.Pt{a, b}
}
