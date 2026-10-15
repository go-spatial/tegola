package proj_test

import (
	"math"
	"sync"
	"testing"

	"github.com/go-spatial/proj"
)

// TestAeaConcurrentConversion covers audit P6-1: Aea.Forward and Aea.Inverse
// used the shared Aea struct field "rho" as scratch space. The operation
// instance is cached per EPSG code and shared across goroutines, so parallel
// conversions silently corrupted each other's geometry. This test runs 8
// goroutines converting different point sets through the SAME cached
// conversion (forward and inverse) and requires every concurrent result to
// match the sequentially computed expectation. Run under go test -race.
func TestAeaConcurrentConversion(t *testing.T) {
	const code proj.EPSGCode = 900010
	proj.CustomProjection(code, "+proj=aea +lat_1=30 +lat_2=60 +lat_0=45 +lon_0=0 +datum=WGS84")
	defer proj.RemoveCustomProjection(code)

	const goroutines = 8
	const pointsPerGoroutine = 250
	const iterations = 20

	// Different point set per goroutine, all inside the projection domain.
	inputs := make([][]float64, goroutines)
	for g := 0; g < goroutines; g++ {
		pts := make([]float64, 0, pointsPerGoroutine*2)
		for i := 0; i < pointsPerGoroutine; i++ {
			k := g*pointsPerGoroutine + i
			lon := -150.0 + float64((k*7)%300)
			lat := 15.0 + float64((k*11)%65)
			pts = append(pts, lon, lat)
		}
		inputs[g] = pts
	}

	// Sequential expectations, computed before the concurrent phase.
	wantFwd := make([][]float64, goroutines)
	wantInv := make([][]float64, goroutines)
	for g := range inputs {
		fwd, err := proj.Convert(code, inputs[g])
		if err != nil {
			t.Fatalf("sequential forward, goroutine %d: %v", g, err)
		}
		inv, err := proj.Inverse(code, fwd)
		if err != nil {
			t.Fatalf("sequential inverse, goroutine %d: %v", g, err)
		}
		wantFwd[g] = fwd
		wantInv[g] = inv
	}

	start := make(chan struct{})
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for iter := 0; iter < iterations; iter++ {
				fwd, err := proj.Convert(code, inputs[g])
				if err != nil {
					t.Errorf("concurrent forward, goroutine %d: %v", g, err)
					return
				}
				if !samePoints(fwd, wantFwd[g]) {
					t.Errorf("concurrent forward, goroutine %d iter %d: results differ from sequential computation", g, iter)
					return
				}
				inv, err := proj.Inverse(code, fwd)
				if err != nil {
					t.Errorf("concurrent inverse, goroutine %d: %v", g, err)
					return
				}
				if !samePoints(inv, wantInv[g]) {
					t.Errorf("concurrent inverse, goroutine %d iter %d: results differ from sequential computation", g, iter)
					return
				}
			}
		}(g)
	}

	close(start)
	wg.Wait()
}

// samePoints reports whether two coordinate arrays match within tolerance.
func samePoints(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			return false
		}
	}
	return true
}
