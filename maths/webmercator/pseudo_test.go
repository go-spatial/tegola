package webmercator

import (
	"math"
	"testing"
)

// valid latitudes must be projected with the exact Web Mercator formula,
// unchanged by the clamping added for out-of-range input.
func TestPLatToYValidLatitudes(t *testing.T) {
	tests := map[string]float64{
		"equator":         0.0,
		"mid north":       45.0,
		"mid south":       -33.9,
		"near edge north": 85.0511,
		"near edge south": -85.0511,
	}

	for name, lat := range tests {
		t.Run(name, func(t *testing.T) {
			rad := DegToRad(lat)
			want := EarthRadius * math.Log(math.Tan(PiDiv4+rad/2))
			got := PLatToY(lat)
			if math.Abs(got-want) > 1e-6 {
				t.Fatalf("PLatToY(%v) = %v, want %v (valid latitudes must not change)", lat, got, want)
			}
		})
	}
}

// Out-of-range latitudes must clamp to the Web Mercator latitude limit instead
// of silently landing on the equator (y = 0) through a NaN in the projection
// math. NaN input propagates as NaN and must never produce y = 0.
func TestPLatToYOutOfRange(t *testing.T) {
	edgeNorth := PLatToY(MaxLatitude)
	edgeSouth := PLatToY(-MaxLatitude)

	tests := map[string]struct {
		lat  float64
		want float64
	}{
		"above 90":     {lat: 91, want: edgeNorth},
		"way above":    {lat: 200, want: edgeNorth},
		"positive inf": {lat: math.Inf(1), want: edgeNorth},
		"below -90":    {lat: -91, want: edgeSouth},
		"way below":    {lat: -200, want: edgeSouth},
		"negative inf": {lat: math.Inf(-1), want: edgeSouth},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := PLatToY(tc.lat)
			if got != tc.want {
				t.Fatalf("PLatToY(%v) = %v, want clamped edge %v", tc.lat, got, tc.want)
			}
			if got == 0 {
				t.Fatalf("PLatToY(%v) = 0: invalid latitude must never land on the equator", tc.lat)
			}
		})
	}

	t.Run("nan", func(t *testing.T) {
		got := PLatToY(math.NaN())
		if !math.IsNaN(got) {
			t.Fatalf("PLatToY(NaN) = %v, want NaN (propagated)", got)
		}
		if got == 0 {
			t.Fatal("PLatToY(NaN) = 0: NaN must never land on the equator")
		}
	})

	t.Run("boundary is unchanged", func(t *testing.T) {
		// exactly at the limit the projection must match the plain formula
		rad := DegToRad(MaxLatitude)
		want := EarthRadius * math.Log(math.Tan(PiDiv4+rad/2))
		if got := PLatToY(MaxLatitude); math.Abs(got-want) > 1e-6 {
			t.Fatalf("PLatToY(MaxLatitude) = %v, want %v", got, want)
		}
		if edgeNorth <= 0 || edgeNorth > 20048966.10 {
			t.Fatalf("north edge %v outside the web mercator extent", edgeNorth)
		}
	})
}

// TestPLonToXNaNPropagation documents the audit P5-14 contract: a NaN
// longitude propagates as a NaN x. The previous behavior logged the problem
// and returned 0, silently drawing invalid input on the central meridian.
func TestPLonToXNaNPropagation(t *testing.T) {
	got := PLonToX(math.NaN())
	if !math.IsNaN(got) {
		t.Fatalf("PLonToX(NaN) = %v, want NaN (invalid input yields invalid output)", got)
	}
	if got == 0 {
		t.Fatal("PLonToX(NaN) = 0: NaN must never collapse to the central meridian")
	}
}

// TestPToXYNaNContract documents the audit P5-14 NaN contract at the point
// level: if either coordinate of a point is NaN, the whole point is invalid
// and BOTH x and y come back as NaN. Finite coordinates and the non-NaN
// clamping behavior are unchanged.
func TestPToXYNaNContract(t *testing.T) {
	nan := math.NaN()

	tests := map[string]struct {
		lon, lat float64
	}{
		"lon is NaN": {lon: nan, lat: 50.0},
		"lat is NaN": {lon: 10.0, lat: nan},
		"both NaN":   {lon: nan, lat: nan},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := PToXY(tc.lon, tc.lat, 13.0, 42.0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !math.IsNaN(got[0]) || !math.IsNaN(got[1]) {
				t.Fatalf("PToXY(%v, %v) = (%v, %v), want (NaN, NaN): an invalid point must be invalid in both coordinates",
					tc.lon, tc.lat, got[0], got[1])
			}
			// z/m pass-through is unaffected by the NaN contract
			if got[2] != 13.0 || got[3] != 42.0 {
				t.Fatalf("z/m values changed: got %v", got[2:])
			}
		})
	}

	t.Run("finite input unchanged", func(t *testing.T) {
		got, err := PToXY(10.0, 50.0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantX := DegToRad(10.0) * EarthRadius
		if math.Abs(got[0]-wantX) > 1e-6 {
			t.Fatalf("PToXY x = %v, want %v", got[0], wantX)
		}
		if math.Abs(got[1]-PLatToY(50.0)) > 1e-6 {
			t.Fatalf("PToXY y = %v, want %v", got[1], PLatToY(50.0))
		}
	})

	t.Run("clamping unchanged", func(t *testing.T) {
		// non-NaN out-of-range latitudes keep clamping to the map edge
		got, err := PToXY(10.0, 91.0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(got[1]-PLatToY(MaxLatitude)) > 1e-6 {
			t.Fatalf("PToXY lat 91 y = %v, want clamped edge %v", got[1], PLatToY(MaxLatitude))
		}
	})
}
