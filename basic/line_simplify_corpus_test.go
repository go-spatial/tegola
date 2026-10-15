package basic_test

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/maths"
	"github.com/alexeydott/tegola/maths/simplify"
)

// Test corpus for line/ring simplification correctness (UPSTREAM.md debt 3.6).
//
// The corpus defines "correct" simplification as these testable invariants,
// checked on every case:
//
//	(a) topology:  a simplified line/ring is non-self-intersecting whenever its
//	    input was (self-intersection = any non-adjacent segment pair sharing a
//	    point, plus adjacent segments overlapping beyond their shared vertex).
//	(b) fidelity:  two-sided discrete Hausdorff distance between input and
//	    output (vertex-to-polyline, both directions, closing edge included for
//	    rings) stays within the simplification tolerance.
//	(c) shape:     endpoints preserved exactly and output is an ordered
//	    subsequence of the input (cyclic subsequence for rings), i.e. the
//	    direction and vertex order are preserved.
//	(d) monotonicity: x-monotone inputs stay x-monotone (control invariant).
//	NaN/Inf-free: inputs are finite by construction and outputs must be finite.
//
// The xfail field documented KNOWN failures of the pre-fix implementation
// (the debt-3.6 bug pins). All markers were removed when the fix landed; the
// runner still rejects stale markers ("STALE XFAIL MARKER" when an xfail case
// passes) and TEGOLA_SIMP_CORPUS_STRICT=1 still turns expected-failure skips
// into test failures, so the corpus can be counted against regressions.

const corpusTolEps = 1e-9

type corpusSurface string

const (
	surfaceDP   corpusSurface = "dp"   // simplify.DouglasPeucker directly
	surfaceLine corpusSurface = "line" // simplify.SimplifyGeometry on basic.Line
	surfaceRing corpusSurface = "ring" // simplify.SimplifyGeometry on basic.Polygon
)

type simplifyCorpusCase struct {
	name    string
	surface corpusSurface
	tol     float64
	// in holds the input vertices. For surfaceRing this is the open loop; the
	// runner closes it with a duplicate of the first point, as the polygon
	// pipeline does.
	in []maths.Pt
	// xfail non-empty marks an expected failure of the implementation with the
	// invariant(s) and mechanism it pins down. Empty since debt 3.6 was fixed;
	// kept so regressions can be pinned with an explicit marker again.
	xfail string
	// wantReduced asserts size reduction: len(out) <= maxRatio*len(in).
	wantReduced bool
	maxRatio    float64
}

// wmBase mimics web-mercator meter magnitudes (~2e7).
const wmBase = -20037508.34

var simplifyCorpus = []simplifyCorpusCase{
	// ---- straight lines & collinear runs (must reduce; trivially safe) ----
	{
		name:        "dp/straight",
		surface:     surfaceDP,
		tol:         0.1,
		in:          []maths.Pt{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 2}, {X: 3, Y: 3}},
		wantReduced: true,
		maxRatio:    0.5,
	},
	{
		name:        "dp/collinear-run",
		surface:     surfaceDP,
		tol:         0.1,
		in:          []maths.Pt{{X: 0, Y: 0}, {X: 2, Y: 4}, {X: 4, Y: 8}, {X: 6, Y: 12}, {X: 8, Y: 16}},
		wantReduced: true,
		maxRatio:    0.5,
	},
	{
		name:        "line/straight-integer",
		surface:     surfaceLine,
		tol:         0.1,
		in:          []maths.Pt{{X: 0, Y: 0}, {X: 2, Y: 2}, {X: 4, Y: 4}, {X: 6, Y: 6}, {X: 8, Y: 8}, {X: 10, Y: 10}},
		wantReduced: true,
		maxRatio:    0.5,
	},

	// ---- zigzags / sawteeth (integers; control cases) ----
	{
		name:    "dp/zigzag-kept",
		surface: surfaceDP,
		tol:     1.5,
		in:      []maths.Pt{{X: 0, Y: 0}, {X: 1, Y: 2}, {X: 2, Y: 0}, {X: 3, Y: 2}, {X: 4, Y: 0}},
	},
	{
		name:    "dp/sawtooth-collapse",
		surface: surfaceDP,
		tol:     2,
		in:      []maths.Pt{{X: 0, Y: 0}, {X: 2, Y: 1}, {X: 4, Y: 0}, {X: 6, Y: 1}, {X: 8, Y: 0}, {X: 10, Y: 1}, {X: 12, Y: 0}},
	},

	// ---- tall thin spikes / needles (fold-backs the naive DP collapses) ----
	{
		name:    "dp/needle-foldback-flat",
		surface: surfaceDP,
		tol:     1,
		// The apex lies exactly on the INFINITE line through the chord
		// (distance 0) but ~990 away from the chord segment: pre-fix DP
		// dropped it and the replacement chord no longer covered the spike.
		in: []maths.Pt{{X: 0, Y: 0}, {X: 1000, Y: 20}, {X: 10, Y: 0.2}},
	},
	{
		name:    "dp/needle-foldback-offset",
		surface: surfaceDP,
		tol:     1,
		in:      []maths.Pt{{X: 0, Y: 0}, {X: 1000, Y: 20.5}, {X: 10, Y: 0.2}},
	},
	{
		name:    "line/needle-geom",
		surface: surfaceLine,
		tol:     1,
		// 5+ points so SimplifyGeometry's len<=4 short-circuit cannot mask it.
		in: []maths.Pt{{X: 0, Y: 0}, {X: 1000, Y: 20.5}, {X: 10, Y: 0.2}, {X: 20, Y: 0.4}, {X: 30, Y: 0.6}},
	},

	// ---- staircases near tolerance (control; must not invent crossings) ----
	{
		name:    "dp/staircase-near-tolerance",
		surface: surfaceDP,
		tol:     0.7,
		in: []maths.Pt{
			{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 3}, {X: 4, Y: 3},
		},
	},

	// ---- self-touching input ((a) inapplicable; (b)/(c)/(d) must hold) ----
	{
		name:    "dp/self-touching",
		surface: surfaceDP,
		tol:     2,
		in:      []maths.Pt{{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 10, Y: 0}, {X: 0, Y: 10}},
	},

	// ---- topology: crossing introduced by the simplification itself ----
	{
		name:    "dp/crossing-created",
		surface: surfaceDP,
		tol:     1.2,
		// Simple input. Pre-fix DP output [(0,0),(10,0),(5,1),(11,-1)]: the
		// replacement chords (0,0)-(10,0) and (5,1)-(11,-1) cross at (8,0),
		// creating a self-intersection out of thin air.
		in: []maths.Pt{{X: 0, Y: 0}, {X: 5, Y: -0.5}, {X: 10, Y: 0}, {X: 5, Y: 1}, {X: 10.5, Y: 0.2}, {X: 11, Y: -1}},
	},

	// ---- large coordinate ranges: web-mercator meters (~2e7) ----
	{
		name:    "dp/large-webmercator",
		surface: surfaceDP,
		tol:     0.25,
		in: []maths.Pt{
			{X: wmBase, Y: wmBase}, {X: wmBase + 13.7, Y: wmBase + 7.95},
			{X: wmBase + 27.4, Y: wmBase + 15.8}, {X: wmBase + 41.1, Y: wmBase + 23.75},
			{X: wmBase + 54.8, Y: wmBase + 31.6}, {X: wmBase + 68.5, Y: wmBase + 39.55},
			{X: wmBase + 82.2, Y: wmBase + 47.4}, {X: wmBase + 95.9, Y: wmBase + 55.35},
		},
	},
	{
		name:    "line/webmercator-truncation",
		surface: surfaceLine,
		tol:     0.1,
		// Fractional meter coordinates: pre-fix simplifyLineString's integer
		// truncation snapped vertices to whole meters (~0.7 shift), far beyond
		// the tolerance.
		in: []maths.Pt{
			{X: wmBase, Y: wmBase}, {X: wmBase + 13.83, Y: wmBase + 7.97},
			{X: wmBase + 27.4, Y: wmBase + 15.8}, {X: wmBase + 41.37, Y: wmBase + 23.75},
			{X: wmBase + 54.8, Y: wmBase + 31.67}, {X: wmBase + 68.43, Y: wmBase + 39.55},
			{X: wmBase + 82.2, Y: wmBase + 47.4}, {X: wmBase + 95.93, Y: wmBase + 55.35},
		},
	},

	// ---- large coordinate ranges: projected degrees ----
	{
		name:    "dp/large-degrees",
		surface: surfaceDP,
		tol:     1e-4,
		in: []maths.Pt{
			{X: 12.345678, Y: 56.789012}, {X: 12.355678, Y: 56.794012},
			{X: 12.365678, Y: 56.799012}, {X: 12.375678, Y: 56.804012},
			{X: 12.385678, Y: 56.809012}, {X: 12.395678, Y: 56.814012},
		},
	},
	{
		name:    "line/degrees-truncation",
		surface: surfaceLine,
		tol:     1e-6,
		// Fractional degree coordinates: pre-fix truncation snapped everything
		// to integer degrees (~100km) and degenerated the line to two
		// identical points.
		in: []maths.Pt{
			{X: 12, Y: 56}, {X: 12.125, Y: 56.0625}, {X: 12.25, Y: 56.125}, {X: 12.375, Y: 56.1875005},
			{X: 12.5, Y: 56.25}, {X: 12.625, Y: 56.3125}, {X: 12.75, Y: 56.375}, {X: 12.875, Y: 56.4375},
		},
	},

	// ---- degenerate inputs (must not panic; outputs must stay finite) ----
	{
		name:    "dp/degenerate-empty",
		surface: surfaceDP,
		tol:     1,
		in:      nil,
	},
	{
		name:    "dp/degenerate-single",
		surface: surfaceDP,
		tol:     1,
		in:      []maths.Pt{{X: 7, Y: 3}},
	},
	{
		name:    "dp/degenerate-pair",
		surface: surfaceDP,
		tol:     1,
		in:      []maths.Pt{{X: 1, Y: 1}, {X: 2, Y: 2}},
	},
	{
		name:        "dp/degenerate-repeated",
		surface:     surfaceDP,
		tol:         1,
		in:          []maths.Pt{{X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}},
		wantReduced: true,
		maxRatio:    0.5,
	},

	// ---- long noisy lines (regression: reduce size, no new self-x) ----
	{
		name:        "dp/long-noisy",
		surface:     surfaceDP,
		tol:         0.4,
		in:          noisyLine(300),
		wantReduced: true,
		maxRatio:    0.5,
	},
	{
		name:        "line/long-noisy-geom",
		surface:     surfaceLine,
		tol:         0.4,
		in:          noisyLineFrac(300),
		wantReduced: true,
		maxRatio:    0.5,
	},

	// ---- rings (polygons): closure, collinear edges, normalizePoints ----
	{
		name:    "ring/simple-convex",
		surface: surfaceRing,
		tol:     0.5,
		in:      []maths.Pt{{X: 4, Y: 0}, {X: 2, Y: 3}, {X: -2, Y: 3}, {X: -4, Y: 0}, {X: -2, Y: -3}, {X: 2, Y: -3}},
	},
	{
		name:    "ring/collinear-edge",
		surface: surfaceRing,
		tol:     0.5,
		in:      []maths.Pt{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 3}, {X: 0, Y: 3}},
	},
	{
		name:    "ring/spike-drop-normalize",
		surface: surfaceRing,
		tol:     1,
		// Pre-fix normalizePoints dropped (10,0) because it was collinear with
		// pts[0] and pts[i+1], even though it is a spike apex w.r.t. its own
		// neighbours; the closing edge (5,20)->(0,0) then stood ~3.16 away
		// from the removed apex. Ring self-intersection/invalidity territory.
		in: []maths.Pt{{X: 5, Y: 20}, {X: 0, Y: 0}, {X: 5, Y: 5}, {X: 10, Y: 0}, {X: 20, Y: 0}, {X: 25, Y: 10}},
	},
	{
		name:    "ring/degrees-truncation",
		surface: surfaceRing,
		tol:     0.05,
		in: []maths.Pt{
			{X: 12.2, Y: 56.4}, {X: 12.6, Y: 56.4}, {X: 12.8, Y: 56.6},
			{X: 12.6, Y: 56.8}, {X: 12.2, Y: 56.8}, {X: 12.0, Y: 56.6},
		},
	},
}

// noisyLine returns a deterministic long x-monotone line: a smooth sine plus
// bounded pseudo-noise, integral x. Used as a size-reduction + topology
// regression.
func noisyLine(n int) []maths.Pt {
	out := make([]maths.Pt, n)
	for i := 0; i < n; i++ {
		out[i] = maths.Pt{
			X: float64(i),
			Y: 3*math.Sin(float64(i)/17) + float64((i*7)%10)/50 - 0.09,
		}
	}
	return out
}

// noisyLineFrac is noisyLine with fractional x; it originally exercised the
// integer truncation the simplify path used to apply.
func noisyLineFrac(n int) []maths.Pt {
	out := noisyLine(n)
	for i := range out {
		out[i].X += 0.13
		out[i].Y += float64((i*3)%5) / 100
	}
	return out
}

func TestSimplifyCorpus(t *testing.T) {
	strict := os.Getenv("TEGOLA_SIMP_CORPUS_STRICT") == "1"
	xfailCount := 0
	for _, c := range simplifyCorpus {
		if c.xfail != "" {
			xfailCount++
		}
		c := c
		t.Run(c.name, func(t *testing.T) {
			in, out, ring := runSimplifyCase(c)
			if !finitePts(in) {
				t.Fatalf("fixture hygiene: input contains NaN/Inf")
			}
			if !finitePts(out) {
				t.Errorf("output contains NaN/Inf")
			}
			violations := checkSimplifyInvariants(c, in, out, ring)
			switch {
			case len(violations) == 0 && c.xfail != "":
				t.Fatalf("STALE XFAIL MARKER (%s): case now satisfies all invariants; flip it to passing", c.xfail)
			case len(violations) > 0 && c.xfail != "":
				msg := fmt.Sprintf("EXPECTED FAILURE (debt 3.6 bug pin) %s | %s", c.xfail, strings.Join(violations, "; "))
				if strict {
					t.Errorf("%s [strict mode]", msg)
				} else {
					t.Skip(msg)
				}
			case len(violations) > 0:
				t.Errorf("invariant violations: %s", strings.Join(violations, "; "))
			}
		})
	}
	t.Logf("simplify corpus: %d cases, %d marked xfail (bug pins)", len(simplifyCorpus), xfailCount)
}

// runSimplifyCase runs one corpus case through its surface and returns the
// input vertices (as seen by the invariants) and the output vertices.
func runSimplifyCase(c simplifyCorpusCase) (in, out []maths.Pt, ring bool) {
	switch c.surface {
	case surfaceDP:
		return c.in, simplify.DouglasPeucker(c.in, c.tol), false
	case surfaceLine:
		g := simplify.SimplifyGeometry(basic.NewLineFromPt(c.in...), c.tol)
		switch v := g.(type) {
		case basic.Line:
			out = v.AsPts()
		case basic.MultiLine:
			if len(v) > 0 {
				out = v[0].AsPts()
			}
		}
		return c.in, out, false
	case surfaceRing:
		closed := make([]maths.Pt, 0, len(c.in)+1)
		closed = append(closed, c.in...)
		closed = append(closed, c.in[0])
		g := simplify.SimplifyGeometry(basic.Polygon{basic.NewLineFromPt(closed...)}, c.tol)
		switch v := g.(type) {
		case basic.Polygon:
			if len(v) > 0 {
				out = v[0].AsPts()
			}
		case basic.MultiPolygon:
			if len(v) > 0 && len(v[0]) > 0 {
				out = v[0][0].AsPts()
			}
		}
		return c.in, loopOpen(out), true
	}
	panic("unknown surface " + string(c.surface))
}

func checkSimplifyInvariants(c simplifyCorpusCase, in, out []maths.Pt, ring bool) (violations []string) {
	// (b) fidelity: two-sided discrete Hausdorff bound.
	if h := hausdorff(in, out, ring); h > c.tol+corpusTolEps {
		violations = append(violations, fmt.Sprintf("(b) hausdorff %.6g > tol %g", h, c.tol))
	}
	// (c) shape: endpoints and direction/order preserved.
	if ring {
		if !cyclicSubsequence(loopOpen(out), loopOpen(in)) {
			violations = append(violations, "(c) output ring is not a cyclic subsequence of the input ring")
		}
	} else {
		if !isSubsequence(out, in) {
			violations = append(violations, "(c) output is not an ordered subsequence of the input")
		}
		if len(in) > 0 && len(out) > 0 {
			if out[0] != in[0] || out[len(out)-1] != in[len(in)-1] {
				violations = append(violations, "(c) endpoints not preserved")
			}
		}
	}
	// (a) topology: no self-intersection invented for simple inputs.
	if !selfIntersecting(in, ring) && selfIntersecting(out, ring) {
		violations = append(violations, "(a) simplified geometry self-intersects but input does not")
	}
	// (d) monotonicity: x-monotone input stays x-monotone.
	if !ring && len(in) > 1 && xMonotone(in) && !xMonotone(out) {
		violations = append(violations, "(d) x-monotone input produced non-monotone output")
	}
	// size reduction regression (long noisy lines, collapse cases).
	if c.wantReduced && len(out) > int(math.Ceil(c.maxRatio*float64(len(in)))) {
		violations = append(violations, fmt.Sprintf("size not reduced enough: %d points > %g%% of %d", len(out), 100*c.maxRatio, len(in)))
	}
	return violations
}

// ---------------------------------------------------------------------------
// geometry predicates (local to the test; deliberately independent from
// production helpers so the corpus validates the implementation from outside)
// ---------------------------------------------------------------------------

// segDistPoint returns the distance from p to the segment a-b.
func segDistPoint(p, a, b maths.Pt) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}

func cross2(u, v maths.Pt) float64 { return u.X*v.Y - u.Y*v.X }

func sub2(a, b maths.Pt) maths.Pt { return maths.Pt{X: a.X - b.X, Y: a.Y - b.Y} }

func dot2(u, v maths.Pt) float64 { return u.X*v.X + u.Y*v.Y }

// samePoint reports whether a and b are equal within a scale-relative epsilon.
func samePoint(a, b maths.Pt) bool {
	const eps = 1e-12
	scale := math.Max(math.Abs(a.X)+math.Abs(a.Y), math.Abs(b.X)+math.Abs(b.Y))
	return math.Abs(a.X-b.X) <= eps*maxFloat(1, scale) &&
		math.Abs(a.Y-b.Y) <= eps*maxFloat(1, scale)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// segSharePoint reports whether segments a-b and c-d share at least one point
// (closed test: proper crossings, endpoint touches and collinear overlaps all
// count).
func segSharePoint(a, b, c, d maths.Pt) bool {
	r := sub2(b, a)
	s := sub2(d, c)
	den := cross2(r, s)
	ca := sub2(c, a)
	scale := math.Sqrt(dot2(r, r) * dot2(s, s))
	if math.Abs(den) > 1e-12*maxFloat(1, scale) {
		t := cross2(ca, s) / den
		u := cross2(ca, r) / den
		return t >= -1e-12 && t <= 1+1e-12 && u >= -1e-12 && u <= 1+1e-12
	}
	// Parallel: share a point only if collinear and their projections overlap.
	if math.Abs(cross2(ca, r)) > 1e-12*maxFloat(1, math.Sqrt(dot2(ca, ca)*dot2(r, r))) {
		return false
	}
	return projOverlap(a, b, c, d)
}

// projOverlap reports whether collinear segments a-b and c-d overlap
// (inclusive: a shared endpoint counts).
func projOverlap(a, b, c, d maths.Pt) bool {
	useX := math.Abs(b.X-a.X) >= math.Abs(b.Y-a.Y)
	minmax := func(p, q maths.Pt) (float64, float64) {
		var u, v float64
		if useX {
			u, v = p.X, q.X
		} else {
			u, v = p.Y, q.Y
		}
		if u > v {
			u, v = v, u
		}
		return u, v
	}
	a0, a1 := minmax(a, b)
	c0, c1 := minmax(c, d)
	return maxFloat(a0, c0) <= math.Min(a1, c1)+1e-12
}

// adjacentOverlap reports whether consecutive segments a-b and b-d overlap
// beyond their shared vertex b (a fold-back), rather than merely meeting.
func adjacentOverlap(a, b, d maths.Pt) bool {
	ba := sub2(a, b)
	bd := sub2(d, b)
	if math.Abs(cross2(ba, bd)) > 1e-12*maxFloat(1, math.Sqrt(dot2(ba, ba)*dot2(bd, bd))) {
		return false // not collinear
	}
	return dot2(ba, bd) > 0 // both unshared ends on the same side of b
}

// selfIntersecting reports whether the polyline (or closed ring when ring is
// true) intersects itself: any non-adjacent segment pair sharing a point, or
// adjacent segments overlapping beyond their shared vertex.
func selfIntersecting(pts []maths.Pt, ring bool) bool {
	n := len(pts)
	if n < 2 {
		return false
	}
	segs := n - 1
	if ring {
		segs = n
	}
	for i := 0; i < segs; i++ {
		a := pts[i]
		b := pts[(i+1)%n]
		for j := i + 1; j < segs; j++ {
			c := pts[j%n]
			d := pts[(j+1)%n]
			switch {
			case j == i+1:
				if adjacentOverlap(a, b, d) {
					return true
				}
			case ring && i == 0 && j == segs-1:
				// closing segment (pts[n-1], pts[0]) adjacent to (pts[0], pts[1])
				if adjacentOverlap(d, a, b) {
					return true
				}
			default:
				if segSharePoint(a, b, c, d) {
					return true
				}
			}
		}
	}
	return false
}

// minDistToPts returns the distance from p to the polyline pts (to the single
// point when pts has fewer than two vertices).
func minDistToPts(p maths.Pt, pts []maths.Pt, ring bool) float64 {
	n := len(pts)
	if n == 0 {
		return math.Inf(1)
	}
	if n == 1 {
		return math.Hypot(p.X-pts[0].X, p.Y-pts[0].Y)
	}
	segs := n - 1
	if ring {
		segs = n
	}
	min := math.Inf(1)
	for i := 0; i < segs; i++ {
		if d := segDistPoint(p, pts[i], pts[(i+1)%n]); d < min {
			min = d
		}
	}
	return min
}

// hausdorff returns the two-sided discrete Hausdorff distance between the two
// polylines (rings: closing edge included on both sides).
func hausdorff(a, b []maths.Pt, ring bool) float64 {
	h := 0.0
	for _, p := range a {
		if d := minDistToPts(p, b, ring); d > h {
			h = d
		}
	}
	for _, p := range b {
		if d := minDistToPts(p, a, ring); d > h {
			h = d
		}
	}
	return h
}

// loopOpen strips the trailing duplicate closing vertex of a closed ring.
func loopOpen(pts []maths.Pt) []maths.Pt {
	if len(pts) >= 2 && samePoint(pts[0], pts[len(pts)-1]) {
		return pts[:len(pts)-1]
	}
	return pts
}

func isSubsequence(sub, seq []maths.Pt) bool {
	i := 0
	for j := 0; i < len(sub) && j < len(seq); j++ {
		if sub[i] == seq[j] {
			i++
		}
	}
	return i == len(sub)
}

// cyclicSubsequence reports whether sub appears in seq as an ordered
// subsequence of some rotation of seq (rings have no canonical start).
func cyclicSubsequence(sub, seq []maths.Pt) bool {
	if len(sub) == 0 {
		return true
	}
	for r := 0; r < len(seq); r++ {
		rot := make([]maths.Pt, 0, len(seq))
		rot = append(rot, seq[r:]...)
		rot = append(rot, seq[:r]...)
		if isSubsequence(sub, rot) {
			return true
		}
	}
	return false
}

func xMonotone(pts []maths.Pt) bool {
	for i := 1; i < len(pts); i++ {
		if pts[i].X < pts[i-1].X {
			return false
		}
	}
	return true
}

func finitePts(pts []maths.Pt) bool {
	for _, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			return false
		}
	}
	return true
}
