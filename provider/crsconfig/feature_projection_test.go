package crsconfig

import (
	"math"
	"strings"
	"sync"
	"testing"
)

func TestFeatureProjectionCanonicalProfilesAndOwnership(t *testing.T) {
	for _, srid := range []uint64{4326, 3857, 32601, 32634, 32660, 32701, 32760} {
		definition, _ := CanonicalFeatureDefinition(srid)
		projection, err := NewFeatureProjection(definition)
		if err != nil {
			t.Fatal(err)
		}
		if projection.CanonicalSRID() != srid || projection.CanonicalDefinition() != definition {
			t.Fatal("canonical identity differs from owned profile")
		}
		canonical, err := NewHeightProjection(srid)
		if err != nil {
			t.Fatal(err)
		}
		input := []float64{sridLongitude(srid), 1}
		want, err := canonical.Forward(input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := projection.Forward(input)
		if err != nil || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("srid%d got%v want%v err%v", srid, got, want, err)
		}
		back, err := projection.Inverse(got)
		if err != nil || math.Abs(back[0]-input[0]) > 1e-9 || math.Abs(back[1]-input[1]) > 1e-9 {
			t.Fatalf("inverse%d %v %v", srid, back, err)
		}
		got[0] = 0
		if input[0] != sridLongitude(srid) {
			t.Fatal("input alias retained")
		}
	}
	definition := "+units=m +datum=WGS84 +zone=34 +proj=utm"
	p, err := NewFeatureProjection(definition)
	if err != nil || p.Definition() != definition {
		t.Fatal(err)
	}
	canonical, _ := CanonicalFeatureDefinition(32634)
	other, _ := NewFeatureProjection(canonical)
	if !p.Equivalent(other) {
		t.Fatal("canonical equivalent parameters differ")
	}
	if p.CanonicalSRID() != 32634 || p.CanonicalDefinition() != canonical {
		t.Fatal("equivalent spelling lost canonical identity")
	}
	geographic, _ := NewFeatureProjection("+proj=longlat +datum=WGS84")
	if p.Equivalent(geographic) || p.Equivalent(nil) {
		t.Fatal("different/unknown mathematics treated equal")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if _, err := p.Forward([]float64{21, 1}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

func sridLongitude(srid uint64) float64 {
	if srid >= 32601 && srid <= 32660 {
		return float64(srid-32600)*6 - 183
	}
	if srid >= 32701 && srid <= 32760 {
		return float64(srid-32700)*6 - 183
	}
	return 21
}

func TestFeatureProjectionRejectsUnprovedParametersAndInvalidCoordinates(t *testing.T) {
	for _, definition := range []string{"", " ", string([]byte{0xff}), "+proj=longlat\x00 +datum=WGS84", strings.Repeat("x", MaxFeatureProjectionDefinitionBytes+1), "+proj=aea +datum=WGS84 +units=m", "+proj=utm +zone=34 +zone=34 +datum=WGS84 +units=m", "+proj=utm +zone=34 +datum=WGS84 +units=m +axis=neu", "+proj=utm +zone=34 +datum=WGS84 +units=m +towgs84=0,0,0", "+proj=utm +zone=34 +datum=WGS84 +units=m +x_0=NaN", "+proj=utm +zone=34 +datum=WGS84 +units=m +no_defs=1"} {
		if _, err := NewFeatureProjection(definition); err == nil {
			t.Fatal("unproved definition accepted")
		}
	}
	p, _ := NewFeatureProjection("+proj=longlat +datum=WGS84")
	for _, input := range [][]float64{{1}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if _, err := p.Forward(input); err == nil {
			t.Fatal("invalid coordinates accepted")
		}
	}
	var absent *FeatureProjection
	if absent.CanonicalSRID() != 0 || absent.CanonicalDefinition() != "" || (&FeatureProjection{}).CanonicalSRID() != 0 {
		t.Fatal("uninitialized projection advertises identity")
	}
	if _, err := absent.Inverse(nil); err == nil {
		t.Fatal("missing converter accepted")
	}
}

func TestClampForwardDomain(t *testing.T) {
	for _, test := range []struct {
		name      string
		srid      uint64
		ll        []float64
		wantLat   float64
		wantClamp bool
	}{
		{"north pole", 3857, []float64{10, 90}, mercatorLatitudeLimit, true},
		{"south pole", 3857, []float64{-30, -90}, -mercatorLatitudeLimit, true},
		{"beyond pole", 3857, []float64{0, 90.5}, mercatorLatitudeLimit, true},
		{"inside domain", 3857, []float64{10, 45}, 0, false},
		{"near pole outside edge", 3857, []float64{10, 89.9}, mercatorLatitudeLimit, true},
		{"nan latitude", 3857, []float64{0, math.NaN()}, 0, false},
		{"geographic never clamps", 4326, []float64{0, 90}, 0, false},
		{"utm never clamps", 32633, []float64{0, 90}, 0, false},
		{"unknown srid", 9999, []float64{0, 90}, 0, false},
		{"bad pair", 3857, []float64{0}, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			clamped, ok := ClampForwardDomain(test.srid, test.ll)
			if ok != test.wantClamp {
				t.Fatalf("clamp=%v want %v", ok, test.wantClamp)
			}
			if !ok {
				return
			}
			if clamped[0] != test.ll[0] || clamped[1] != test.wantLat {
				t.Fatalf("clamped=%v", clamped)
			}
			// The clamped vertex must forward without error and land inside
			// the canonical Web Mercator world extent.
			def, _ := CanonicalFeatureDefinition(test.srid)
			p, err := NewFeatureProjection(def)
			if err != nil {
				t.Fatal(err)
			}
			xy, err := p.Forward(clamped)
			if err != nil {
				t.Fatalf("clamped vertex still fails: %v", err)
			}
			if math.Abs(xy[1]) > mercatorWorldExtent {
				t.Fatalf("clamped image %v outside canonical extent ±%v", xy[1], mercatorWorldExtent)
			}
		})
	}
}
