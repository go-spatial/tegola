package hana

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
)

func mustProjectPoint(t *testing.T, srid uint64, x, y float64) geom.Point {
	t.Helper()
	g, err := basic.FromWebMercator(srid, geom.Point{x, y})
	if err != nil {
		t.Fatalf("FromWebMercator(%v, %v) errored = %v", x, y, err)
	}
	pt, ok := g.(geom.Point)
	if !ok {
		t.Fatalf("transformed geometry is %T, expected geom.Point", g)
	}
	return pt
}

// TestGetBBoxCoordinatesCoversFullPerimeter pins audit P5-15: the tile
// extent must be transformed along its full perimeter (min/max taken),
// not just via the two opposite corners. In rotated or polar projections
// the other two corner images bow outside the naive two-corner box, so the
// old conversion under-covered the tile footprint and the bbox predicate
// dropped valid features. Pre-fix: the returned box is exactly the naive
// two-corner box and at least one corner image lies outside it.
func TestGetBBoxCoordinatesCoversFullPerimeter(t *testing.T) {
	for name, tc := range map[string]struct {
		srid uint64
		proj string
	}{
		// UTM zones give converged-meridian (rotated) transforms - the same
		// class of non-axis-aligned mapping as polar projections. The
		// vendored proj4 rejects polar definitions (stere/aeqd/lcc fail its
		// round-trip probe), so the rotated cases are exercised via UTM.
		"utm zone 33": {9932633, "+proj=utm +zone=33 +datum=WGS84 +units=m"},
		"utm zone 31": {9932631, "+proj=utm +zone=31 +datum=WGS84 +units=m"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := basic.RegisterProj4SRID(tc.srid, tc.proj); err != nil {
				t.Fatalf("RegisterProj4SRID(%d) errored = %v", tc.srid, err)
			}
			extent := geom.NewExtent([2]float64{-1e6, 7e6}, [2]float64{2e6, 9e6})

			ll, ur, err := getBBoxCoordinates(extent, tc.srid)
			if err != nil {
				t.Fatalf("getBBoxCoordinates errored = %v", err)
			}

			// The naive two-corner conversion (pre-fix behavior) for comparison.
			minPt := mustProjectPoint(t, tc.srid, extent.MinX(), extent.MinY())
			maxPt := mustProjectPoint(t, tc.srid, extent.MaxX(), extent.MaxY())
			naiveMinX := math.Min(minPt.X(), maxPt.X())
			naiveMaxX := math.Max(minPt.X(), maxPt.X())
			naiveMinY := math.Min(minPt.Y(), maxPt.Y())
			naiveMaxY := math.Max(minPt.Y(), maxPt.Y())

			const eps = 1e-6
			outsideNaive := 0
			for _, c := range [][2]float64{
				{extent.MinX(), extent.MinY()},
				{extent.MinX(), extent.MaxY()},
				{extent.MaxX(), extent.MinY()},
				{extent.MaxX(), extent.MaxY()},
			} {
				pt := mustProjectPoint(t, tc.srid, c[0], c[1])
				if pt.X() < ll.X()-eps || pt.X() > ur.X()+eps || pt.Y() < ll.Y()-eps || pt.Y() > ur.Y()+eps {
					t.Errorf("corner image %v lies outside the computed bbox [%v, %v]: the bbox must cover the whole transformed tile footprint", pt, ll, ur)
				}
				if pt.X() < naiveMinX-eps || pt.X() > naiveMaxX+eps || pt.Y() < naiveMinY-eps || pt.Y() > naiveMaxY+eps {
					outsideNaive++
				}
			}
			if outsideNaive == 0 {
				t.Error("expected at least one corner image outside the naive two-corner box (projection must not be axis-aligned for this test to be meaningful)")
			}
			// The computed bbox is a superset of the naive box on every axis.
			if ll.X() > naiveMinX+eps || ur.X() < naiveMaxX-eps || ll.Y() > naiveMinY+eps || ur.Y() < naiveMaxY-eps {
				t.Errorf("computed bbox [%v, %v] must contain the naive two-corner box [(%v, %v), (%v, %v)]", ll, ur, naiveMinX, naiveMinY, naiveMaxX, naiveMaxY)
			}
		})
	}
}
