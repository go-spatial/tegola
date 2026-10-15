// Copyright 2025 The Tegola Authors. All rights reserved.
// Use of this source code is governed by an MIT-style license
// that can be found in the LICENSE file.

package simplify_test

import (
	"context"
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/maths"
	"github.com/alexeydott/tegola/maths/simplify"
	"google.golang.org/protobuf/proto"
)

// Regression for UPSTREAM.md debt 3.6 at the MVT encoder boundary: a simplified
// tile geometry must not contain a self-intersecting line, must preserve its
// endpoints, and must not silently drop needle vertices. The test replays the
// production encode sequence (the atlas/map.go encodeMVTFeature path):
//
//	simplify.SimplifyGeometry -> mvt.PrepareGeo -> mvt.Feature -> proto bytes -> decode
//
// The fixtures are integer coordinates inside a 4096x4096 extent, so the
// pixel transform is the identity and cursor quantisation is exact; any
// deviation therefore comes from simplification itself.
func TestSimplifyEncodeNoSelfIntersection(t *testing.T) {
	// 4096x4096 span -> PrepareGeo scale factor is exactly 1 in both axes.
	ext := geom.NewExtent([2]float64{0, -400}, [2]float64{4096, 3696})

	for _, tc := range encodeRegressionCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if !tc.inSelfTouches && encSelfIntersecting(tc.in) {
				t.Fatal("fixture input unexpectedly self-intersects; fix the fixture")
			}

			// production simplification surface (atlas/map.go encodeMVTFeature).
			g := simplify.SimplifyGeometry(basic.NewLineFromPt(tc.in...), tc.tol)
			var simplified []maths.Pt
			switch v := g.(type) {
			case basic.Line:
				simplified = v.AsPts()
			case basic.MultiLine:
				if len(v) > 0 {
					simplified = v[0].AsPts()
				}
			default:
				t.Fatalf("simplification returned unexpected geometry type %T", g)
			}

			if len(simplified) < 2 {
				t.Fatalf("simplification returned %d points, want >= 2", len(simplified))
			}
			if !encSubsequence(simplified, tc.in) {
				t.Errorf("simplified vertices are not a subsequence of the input (points invented or reordered)")
			}
			if !encSamePt(simplified[0], tc.in[0]) || !encSamePt(simplified[len(simplified)-1], tc.in[len(tc.in)-1]) {
				t.Errorf("simplified endpoints changed: got %v..%v want %v..%v",
					simplified[0], simplified[len(simplified)-1], tc.in[0], tc.in[len(tc.in)-1])
			}
			if !tc.inSelfTouches && encSelfIntersecting(simplified) {
				t.Errorf("simplification created a self-intersection (needles): %v", simplified)
			}
			if tc.wantApex != nil && !encContains(simplified, *tc.wantApex) {
				t.Errorf("needle apex %v dropped from simplified line %v", *tc.wantApex, simplified)
			}
			if tc.wantMaxCount > 0 && len(simplified) > tc.wantMaxCount {
				t.Errorf("simplification did not compress: %d points, want <= %d", len(simplified), tc.wantMaxCount)
			}

			// production encode sequence.
			prepIn := mvt.PrepareGeo(encGeomLine(tc.in), ext, float64(mvt.DefaultExtent))
			prepOut := mvt.PrepareGeo(encGeomLine(simplified), ext, float64(mvt.DefaultExtent))

			layer := &mvt.Layer{Name: "regression"}
			layer.AddFeatures(mvt.Feature{Geometry: prepOut, Tags: map[string]interface{}{}})
			var tile mvt.Tile
			if err := tile.AddLayers(layer); err != nil {
				t.Fatalf("AddLayers: %v", err)
			}
			vtile, err := tile.VTile(context.Background())
			if err != nil {
				t.Fatalf("VTile: %v", err)
			}
			buf, err := proto.Marshal(vtile)
			if err != nil {
				t.Fatalf("marshal tile: %v", err)
			}
			decoded, err := mvt.DecodeByte(buf)
			if err != nil {
				t.Fatalf("decode tile: %v", err)
			}

			coll := mvt.TileGeomCollection(decoded)
			if len(coll) != 1 {
				t.Fatalf("decoded %d geometries, want 1", len(coll))
			}
			dec := encSingleLine(t, coll[0])

			if !encFinite(dec) {
				t.Errorf("decoded line contains non-finite coordinates: %v", dec)
			}
			if !encSubsequence(dec, prepInPts(prepIn)) {
				t.Errorf("decoded vertices are not a subsequence of the encoded input (quantisation drifted)")
			}
			if !tc.inSelfTouches && encSelfIntersecting(dec) {
				t.Errorf("decoded tile line self-intersects: %v", dec)
			}
		})
	}
}

type encodeRegressionCase struct {
	name string
	in   []maths.Pt
	tol  float64
	// inSelfTouches: the input already folds back onto itself; invariant (a)
	// only applies when the input is simple.
	inSelfTouches bool
	// wantApex pins a vertex that must survive simplification (needle case).
	wantApex *maths.Pt
	// wantMaxCount, when > 0, bounds the output size (compression sanity).
	wantMaxCount int
}

var encodeRegressionCases = []encodeRegressionCase{
	{
		// Pre-fix, the middle vertex here was dropped and the replacement
		// chord crossed an earlier segment at (800, 0): a self-intersecting
		// tile line.
		name: "crossing-created",
		in: []maths.Pt{
			{X: 0, Y: 0}, {X: 500, Y: -50}, {X: 1000, Y: 0},
			{X: 500, Y: 100}, {X: 1050, Y: 20}, {X: 1100, Y: -100},
		},
		tol: 120,
	},
	{
		// Collinear needle: pre-fix infinite-line distance saw every vertex as
		// distance 0 and deleted the 3000-unit spike entirely.
		name:          "needle-foldback",
		in:            []maths.Pt{{X: 0, Y: 0}, {X: 3000, Y: 600}, {X: 30, Y: 6}, {X: 60, Y: 12}, {X: 90, Y: 18}},
		tol:           4,
		inSelfTouches: true,
		wantApex:      &maths.Pt{X: 3000, Y: 600},
	},
	{
		// Long x-monotone noisy line: compression must survive the encoder
		// round trip without inventing crossings.
		name:         "long-noisy-roundtrip",
		in:           encNoisyInts(300),
		tol:          40,
		wantMaxCount: 150,
	},
}

// encNoisyInts mirrors the corpus noisyLine fixture at 100x scale with
// integer coordinates so cursor quantisation is exact.
func encNoisyInts(n int) []maths.Pt {
	pts := make([]maths.Pt, n)
	for i := range pts {
		pts[i] = maths.Pt{
			X: float64(i) * 10,
			Y: math.Round(300*math.Sin(float64(i)/17)) + float64(((i*7)%10)*2) - 9,
		}
	}
	return pts
}

func encGeomLine(pts []maths.Pt) geom.LineString {
	ls := make(geom.LineString, len(pts))
	for i, p := range pts {
		ls[i] = [2]float64{p.X, p.Y}
	}
	return ls
}

func prepInPts(g geom.Geometry) []maths.Pt {
	return encGeomPts(g.(geom.LineString))
}

func encGeomPts(ls geom.LineString) []maths.Pt {
	pts := make([]maths.Pt, len(ls))
	for i, p := range ls {
		pts[i] = maths.Pt{X: p[0], Y: p[1]}
	}
	return pts
}

func encSingleLine(t *testing.T, g geom.Geometry) []maths.Pt {
	t.Helper()
	switch v := g.(type) {
	case geom.LineString:
		return encGeomPts(v)
	case geom.MultiLineString:
		if len(v) != 1 {
			t.Fatalf("decoded %d line parts, want 1", len(v))
		}
		return encGeomPts(v[0])
	default:
		t.Fatalf("decoded unexpected geometry type %T", g)
		return nil
	}
}

func encSamePt(a, b maths.Pt) bool {
	const eps = 1e-9
	return math.Abs(a.X-b.X) <= eps && math.Abs(a.Y-b.Y) <= eps
}

func encContains(pts []maths.Pt, want maths.Pt) bool {
	for _, p := range pts {
		if encSamePt(p, want) {
			return true
		}
	}
	return false
}

func encFinite(pts []maths.Pt) bool {
	for _, p := range pts {
		if math.IsNaN(p.X) || math.IsInf(p.X, 0) || math.IsNaN(p.Y) || math.IsInf(p.Y, 0) {
			return false
		}
	}
	return true
}

// encSubsequence reports whether out is an ordered subset of in
// (endpoints included); this is invariant (c).
func encSubsequence(out, in []maths.Pt) bool {
	j := 0
	for _, p := range out {
		for j < len(in) && !encSamePt(p, in[j]) {
			j++
		}
		if j == len(in) {
			return false
		}
		j++
	}
	return len(out) > 0
}

func encCross(a, b, c maths.Pt) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func encDot(a, b, c maths.Pt) float64 {
	return (b.X-a.X)*(c.X-a.X) + (b.Y-a.Y)*(c.Y-a.Y)
}

// encSegCross reports a proper interior crossing of segments p1-p2 and p3-p4.
func encSegCross(p1, p2, p3, p4 maths.Pt) bool {
	const eps = 1e-12
	scale := math.Max(1, math.Max(math.Abs(p2.X-p1.X)+math.Abs(p2.Y-p1.Y), math.Abs(p4.X-p3.X)+math.Abs(p4.Y-p3.Y)))
	d1 := encCross(p3, p4, p1)
	d2 := encCross(p3, p4, p2)
	d3 := encCross(p1, p2, p3)
	d4 := encCross(p1, p2, p4)
	zero := func(v float64) bool { return math.Abs(v) <= eps*scale*scale }
	if (zero(d1) || zero(d2)) && (zero(d3) || zero(d4)) {
		return false // touching or collinear endpoints: not a proper crossing
	}
	return (d1 > 0) != (d2 > 0) && (d3 > 0) != (d4 > 0)
}

// encFoldBack reports adjacent segments b->a and b->c doubling back over the
// same line from shared vertex b.
func encFoldBack(a, b, c maths.Pt) bool {
	const eps = 1e-12
	ba := maths.Pt{X: a.X - b.X, Y: a.Y - b.Y}
	bc := maths.Pt{X: c.X - b.X, Y: c.Y - b.Y}
	cross := ba.X*bc.Y - ba.Y*bc.X
	scale := math.Max(1, math.Sqrt((ba.X*ba.X+ba.Y*ba.Y)*(bc.X*bc.X+bc.Y*bc.Y)))
	return math.Abs(cross) <= eps*scale && ba.X*bc.X+ba.Y*bc.Y > 0
}

// encSelfIntersecting reports whether the open polyline crosses itself:
// any non-adjacent segment pair properly crossing, or adjacent segments
// doubling back over each other.
func encSelfIntersecting(pts []maths.Pt) bool {
	n := len(pts)
	if n < 3 {
		return false
	}
	for i := 0; i < n-2; i++ {
		if encFoldBack(pts[i], pts[i+1], pts[i+2]) {
			return true
		}
	}
	for i := 0; i < n-1; i++ {
		for j := i + 2; j < n-1; j++ {
			if encSegCross(pts[i], pts[i+1], pts[j], pts[j+1]) {
				return true
			}
		}
	}
	return false
}
