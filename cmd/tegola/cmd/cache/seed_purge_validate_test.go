package cache

import (
	"strings"
	"testing"
)

// withSeedPurgeFlags points the package-level seed/purge flag variables at the
// given values for the duration of a test.
func withSeedPurgeFlags(t *testing.T, concurrency int, bounds string, boundsSRID int, minZ, maxZ uint) {
	t.Helper()
	oldConcurrency := cacheConcurrency
	oldBounds := cacheBounds
	oldBoundsSRID := cacheBoundsSRID
	oldMinZoom := minZoom
	oldMaxZoom := maxZoom
	cacheConcurrency = concurrency
	cacheBounds = bounds
	cacheBoundsSRID = boundsSRID
	minZoom = minZ
	maxZoom = maxZ
	t.Cleanup(func() {
		cacheConcurrency = oldConcurrency
		cacheBounds = oldBounds
		cacheBoundsSRID = oldBoundsSRID
		minZoom = oldMinZoom
		maxZoom = oldMaxZoom
	})
}

// TestSeedPurgeCmdValidateBounds ensures --bounds is interpreted and
// validated against the declared --bounds-srid and that inverted bounds
// (min > max on either axis) are rejected (part13 P6-26).
func TestSeedPurgeCmdValidateBounds(t *testing.T) {
	metric := 20037508.342789244
	tests := []struct {
		name         string
		bounds       string
		srid         int
		expectErr    string // substring of the expected error; empty means valid
		expectBounds [4]float64
	}{
		{
			name: "4326 full globe degrees", bounds: "-180,-90,180,90", srid: 4326,
			expectBounds: [4]float64{-180, -90, 180, 90},
		},
		{
			name: "4326 local degrees", bounds: "-122.4,37.7,-122.3,37.8", srid: 4326,
			expectBounds: [4]float64{-122.4, 37.7, -122.3, 37.8},
		},
		{
			name: "4326 lng out of range", bounds: "181,0,182,1", srid: 4326,
			expectErr: "lng",
		},
		{
			name: "4326 lat out of range", bounds: "0,91,1,92", srid: 4326,
			expectErr: "lat",
		},
		{
			name: "3857 metric world extent", bounds: "-20037508.342789244,-20037508.342789244,20037508.342789244,20037508.342789244", srid: 3857,
			expectBounds: [4]float64{-metric, -metric, metric, metric},
		},
		{
			name: "3857 small metric values", bounds: "1000,2000,3000,4000", srid: 3857,
			expectBounds: [4]float64{1000, 2000, 3000, 4000},
		},
		{
			name: "3857 x out of metric range", bounds: "-20037508.4,0,20037508.4,1", srid: 3857,
			expectErr: "x",
		},
		{
			name: "3857 y out of metric range", bounds: "0,-20037508.4,1,20037508.4", srid: 3857,
			expectErr: "y",
		},
		{
			name: "4326 inverted x", bounds: "10,0,0,10", srid: 4326,
			expectErr: "greater than",
		},
		{
			name: "4326 inverted y", bounds: "0,10,10,0", srid: 4326,
			expectErr: "greater than",
		},
		{
			name: "3857 inverted x", bounds: "1000,0,0,1000", srid: 3857,
			expectErr: "greater than",
		},
		{
			name: "3857 inverted y", bounds: "0,1000,1000,0", srid: 3857,
			expectErr: "greater than",
		},
		{
			name: "wrong number of parts", bounds: "1,2,3", srid: 4326,
			expectErr: "expecting",
		},
		{
			name: "non numeric", bounds: "a,b,c,d", srid: 4326,
			expectErr: "invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withSeedPurgeFlags(t, 1, tc.bounds, tc.srid, 0, 2)
			err := seedPurgeCmdValidate(nil, nil)
			if tc.expectErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q for bounds (%s, srid %d), got nil", tc.expectErr, tc.bounds, tc.srid)
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Errorf("expected error containing %q, got: %v", tc.expectErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected bounds (%s, srid %d) to be valid, got: %v", tc.bounds, tc.srid, err)
			}
			if seedPurgeBounds != tc.expectBounds {
				t.Errorf("parsed bounds mismatch: got %v, want %v", seedPurgeBounds, tc.expectBounds)
			}
		})
	}
}

// TestSeedPurgeCmdValidateConcurrency ensures --concurrency is rejected when
// below 1: 0 would leave the seeder with no workers (it hangs on wg.Wait) and
// negative values panic (part13 P6-24).
func TestSeedPurgeCmdValidateConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		expectErr   bool
	}{
		{name: "zero hangs", concurrency: 0, expectErr: true},
		{name: "negative panics", concurrency: -1, expectErr: true},
		{name: "minimum valid", concurrency: 1, expectErr: false},
		{name: "many workers", concurrency: 16, expectErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withSeedPurgeFlags(t, tc.concurrency, "-180,-85.0511,180,85.0511", 4326, 0, 2)
			err := seedPurgeCmdValidate(nil, nil)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for concurrency %d, got nil", tc.concurrency)
				}
				if !strings.Contains(err.Error(), "concurrency") {
					t.Errorf("expected error to mention concurrency, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error for concurrency %d, got: %v", tc.concurrency, err)
			}
		})
	}
}
