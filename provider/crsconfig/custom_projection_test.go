package crsconfig

import (
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/proj"
)

const customTestDefinition = "+proj=etmerc +lat_0=50 +lon_0=30 +k=1 +x_0=1000 +y_0=2000 +ellps=bessel +towgs84=10,20,30,0.1,-0.2,0.3,1 +units=m"

func TestCustomFeatureProjectionOracle(t *testing.T) {
	p, err := NewFeatureProjection(customTestDefinition)
	if err != nil {
		t.Fatal(err)
	}
	// Independent PROJ 9.5.1 / pyproj 3.7.2, always_xy, WGS84 -> synthetic CRS.
	xy, err := p.Forward([]float64{30.2, 50.1, 31, 51})
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{15289.293080097545, 13063.613876462117, 71171.15093657706, 113625.65923441145}
	for i := range want {
		if math.Abs(xy[i]-want[i]) > 0.001 {
			t.Fatalf("coordinate %d: %.12f want %.12f", i, xy[i], want[i])
		}
	}
	ll, err := p.Inverse([]float64{1000, 2000})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(ll[0]-30.00025960311296) > 1e-9 || math.Abs(ll[1]-50.00069571970949) > 1e-9 {
		t.Fatalf("inverse %v", ll)
	}
	if p.CanonicalSRID() != 0 || p.CanonicalDefinition() != "" {
		t.Fatal("custom definition claims canonical authority")
	}
}

func TestCustomFeatureProjectionUnitsMeridianAndThreeParameterDatum(t *testing.T) {
	p, err := NewFeatureProjection("+proj=etmerc +lat_0=50 +lon_0=30 +k=1 +x_0=1000 +y_0=2000 +ellps=bessel +towgs84=10,20,30 +units=km +pm=paris")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Forward([]float64{32.5, 50.1})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got[0]-12.632989942486232) > 1e-6 || math.Abs(got[1]-13.064400982789305) > 1e-6 {
		t.Fatalf("forward %v", got)
	}
	back, err := p.Inverse([]float64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(back[0]-32.337390272838306) > 1e-9 || math.Abs(back[1]-50.00063062604668) > 1e-9 {
		t.Fatalf("inverse %v", back)
	}
}

func TestCustomFeatureProjectionAdmissionAndIdentity(t *testing.T) {
	p, err := NewFeatureProjection(customTestDefinition)
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(customTestDefinition)
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	same, err := NewFeatureProjection(strings.Join(words, " ") + " +no_defs")
	if err != nil || !p.Equivalent(same) {
		t.Fatalf("equivalent definition rejected: %v", err)
	}
	different, err := NewFeatureProjection(strings.Replace(customTestDefinition, "+lon_0=30", "+lon_0=31", 1))
	if err != nil || p.Equivalent(different) {
		t.Fatal("custom profiles share false identity")
	}
	for _, suffix := range []string{" +axis=neu", " +nadgrids=@null", " +geoidgrids=test", " +foo=bar", " +k_0=2", " +to_meter=2", " +vunits=m", " +zone=30", " +lat_ts=50"} {
		if _, err := NewFeatureProjection(customTestDefinition + suffix); err == nil {
			t.Fatalf("unsupported semantics accepted: %s", suffix)
		}
	}
	for _, datum := range []string{"1,2", "1,2,NaN", "1,2,3,0,0,0,-1000000"} {
		definition := strings.Replace(customTestDefinition, "10,20,30,0.1,-0.2,0.3,1", datum, 1)
		if _, err := NewFeatureProjection(definition); err == nil {
			t.Fatalf("bad datum accepted: %s", datum)
		}
	}
	for _, coords := range [][]float64{{1, 2, 3}, {1, 91}, {math.Inf(1), 2}} {
		if _, err := p.Forward(coords); err == nil {
			t.Fatalf("bad coordinates accepted %v", coords)
		}
	}
	// Custom horizontal support cannot be used as proof of a canonical 3D CRS.
	if _, err := NewHeightProjection(p.CanonicalSRID()); err == nil {
		t.Fatal("custom projection admitted as height preserving")
	}
}

func TestCustomFeatureProjectionRegistryIsolationAndConcurrency(t *testing.T) {
	const code = proj.EPSGCode(998877)
	defer proj.RemoveCustomProjection(code)
	p, err := NewFeatureProjection(customTestDefinition)
	if err != nil {
		t.Fatal(err)
	}
	input := []float64{30.2, 50.1}
	want, err := p.Forward(input)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				proj.CustomProjection(code, "+proj=utm +zone=31 +datum=WGS84")
				got, err := p.Forward(input)
				if err != nil || got[0] != want[0] || got[1] != want[1] {
					t.Errorf("owned projection changed: %v %v", got, err)
					return
				}
				if _, err := p.Inverse(got); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if input[0] != 30.2 || input[1] != 50.1 {
		t.Fatal("input mutated")
	}
}
