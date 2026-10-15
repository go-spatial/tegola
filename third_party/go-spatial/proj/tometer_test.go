package proj_test

import (
	"math"
	"testing"

	"github.com/go-spatial/proj"
)

// TestToMeterUnitScaling covers audit P6-22: +to_meter must be honored as a
// linear unit multiplier (one unit = to_meter meters), per PROJ semantics.
// Previously the code looked up a nonexistent "toMeter" key, so +to_meter was
// rejected outright (and the from-factor was computed from a stale value).
func TestToMeterUnitScaling(t *testing.T) {
	const codeMeters proj.EPSGCode = 900001
	const codeKiloMeters proj.EPSGCode = 900002

	proj.CustomProjection(codeMeters, "+proj=merc +ellps=WGS84 +units=m")
	proj.CustomProjection(codeKiloMeters, "+proj=merc +ellps=WGS84 +units=m +to_meter=1000")
	defer proj.RemoveCustomProjection(codeMeters)
	defer proj.RemoveCustomProjection(codeKiloMeters)

	point := []float64{10.0, 50.0}

	outM, err := proj.Convert(codeMeters, point)
	if err != nil {
		t.Fatalf("convert with +units=m: %v", err)
	}
	outKM, err := proj.Convert(codeKiloMeters, point)
	if err != nil {
		t.Fatalf("convert with +to_meter=1000: %v", err)
	}

	// Same projection, so coordinates must differ exactly by the unit factor.
	for i := range outM {
		want := outM[i] / 1000.0
		if math.Abs(outKM[i]-want) > 1e-9*math.Max(1, math.Abs(want)) {
			t.Errorf("forward coord %d: got %v, want %v (= %v/1000)", i, outKM[i], want, outM[i])
		}
	}

	// Forward then inverse through the +to_meter system must round-trip.
	back, err := proj.Inverse(codeKiloMeters, outKM)
	if err != nil {
		t.Fatalf("inverse with +to_meter=1000: %v", err)
	}
	for i := range point {
		if math.Abs(back[i]-point[i]) > 1e-9 {
			t.Errorf("round-trip coord %d: got %v, want %v", i, back[i], point[i])
		}
	}
}

// TestToMeterWithoutUnits checks that +to_meter works without a +units key
// and that inverse scaling is applied as the exact reciprocal.
func TestToMeterWithoutUnits(t *testing.T) {
	const codeKiloMeters proj.EPSGCode = 900003

	proj.CustomProjection(codeKiloMeters, "+proj=merc +ellps=WGS84 +to_meter=1000")
	defer proj.RemoveCustomProjection(codeKiloMeters)

	point := []float64{-73.9857, 40.7484}
	out, err := proj.Convert(codeKiloMeters, point)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	back, err := proj.Inverse(codeKiloMeters, out)
	if err != nil {
		t.Fatalf("inverse: %v", err)
	}
	for i := range point {
		if math.Abs(back[i]-point[i]) > 1e-9 {
			t.Errorf("round-trip coord %d: got %v, want %v", i, back[i], point[i])
		}
	}
}

// TestToMeterInvalidFactor rejects non-positive unit factors instead of
// producing degenerate scaling.
func TestToMeterInvalidFactor(t *testing.T) {
	const codeBad proj.EPSGCode = 900004

	proj.CustomProjection(codeBad, "+proj=merc +ellps=WGS84 +to_meter=0")
	defer proj.RemoveCustomProjection(codeBad)

	if _, err := proj.Convert(codeBad, []float64{10.0, 50.0}); err == nil {
		t.Fatal("expected error for +to_meter=0, got nil")
	}
}
