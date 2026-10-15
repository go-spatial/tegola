package crsconfig

import (
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/alexeydott/proj"
)

func TestHeightProjectionIndependentOracles(t *testing.T) {
	// Independent scalar Mercator and Snyder central-meridian meridional-arc
	// oracles frozen during G2. The truncated UTM series and converter differ
	// by about 27 micrometres; the declared metric tolerance is 0.1 millimetre.
	for _, test := range []struct {
		srid uint64
		xy   []float64
	}{
		{4326, []float64{15, 30}},
		{3857, []float64{1669792.3618991037, 3503549.843504374}},
		{32633, []float64{500000, 3318785.352608442}},
		{32733, []float64{500000, 6681214.647391558}},
	} {
		p, err := NewHeightProjection(test.srid)
		if err != nil {
			t.Fatal(err)
		}
		latitude := float64(30)
		if test.srid == 32733 {
			latitude = -30
		}
		input := []float64{15, latitude}
		before := append([]float64(nil), input...)
		got, err := p.Forward(input)
		if err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if math.IsNaN(got[i]) || math.IsInf(got[i], 0) || math.Abs(got[i]-test.xy[i]) > 1e-4 {
				t.Fatalf("SRID %d axis%d got%.12f want%.12f", test.srid, i, got[i], test.xy[i])
			}
		}
		if !reflect.DeepEqual(input, before) {
			t.Fatal("input mutated")
		}
		inverse, err := p.Inverse(test.xy)
		if err != nil {
			t.Fatal(err)
		}
		for i := range inverse {
			if math.IsNaN(inverse[i]) || math.IsInf(inverse[i], 0) || math.Abs(inverse[i]-input[i]) > 1e-9 {
				t.Fatalf("SRID%d inverse %v", test.srid, inverse)
			}
		}
	}
}

func TestHeightProjectionRegistryIsolation(t *testing.T) {
	p, err := NewHeightProjection(32633)
	if err != nil {
		t.Fatal(err)
	}
	// This test owns an otherwise unregistered code and restores it afterwards.
	defer proj.RemoveCustomProjection(32633)
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			proj.CustomProjection(32633, "+proj=utm +zone=34 +datum=WGS84")
		}
	})
	for range 100 {
		got, err := p.Forward([]float64{15, 30})
		if err != nil || math.Abs(got[0]-500000) > 1e-6 {
			t.Fatalf("mutable registry affected snapshot: %v %v", got, err)
		}
	}
	wg.Wait()
	other, err := NewHeightProjection(32633)
	if err != nil || other.Definition() != p.Definition() {
		t.Fatal("new canonical instance depends on registry")
	}
}

func TestHeightProjectionValidation(t *testing.T) {
	for _, srid := range []uint64{0, 3395, 32600, 32661, 32700, 32761, 340000001} {
		if _, err := NewHeightProjection(srid); err == nil {
			t.Fatalf("accepted%d", srid)
		}
	}
	p, err := NewHeightProjection(3857)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range [][]float64{{1}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if _, err := p.Forward(input); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
	var nilProjection *HeightProjection
	if _, err := nilProjection.Inverse(nil); err == nil {
		t.Fatal("nil projection accepted")
	}
	var zero HeightProjection
	if _, err := zero.Forward([]float64{0, 0}); err == nil {
		t.Fatal("uninitialized projection accepted")
	}
}

func TestHeightProjectionConcurrentCalls(t *testing.T) {
	p, err := NewHeightProjection(3857)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				got, err := p.Forward([]float64{0, 0})
				if err != nil || len(got) != 2 || math.Abs(got[0]) > 1e-8 || math.Abs(got[1]) > 1e-8 {
					t.Errorf("concurrent forward: %v %v", got, err)
				}
				inverse, err := p.Inverse([]float64{0, 0})
				if err != nil || len(inverse) != 2 || math.Abs(inverse[0]) > 1e-8 || math.Abs(inverse[1]) > 1e-8 {
					t.Errorf("concurrent inverse: %v %v", inverse, err)
				}
			}
		})
	}
	wg.Wait()
}
