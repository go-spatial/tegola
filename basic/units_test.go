package basic

import (
	"math"
	"testing"
)

func TestProjectedMetersPerUnit(t *testing.T) {
	for _, tc := range []struct {
		name, units string
		want        float64
	}{
		{"meters", "+units=m", 1},
		{"default meters", "", 1},
		{"feet", "+units=ft", 0.3048},
		{"US survey feet", "+units=us-ft", 1200.0 / 3937},
		{"kilometers", "+units=km", 1000},
		{"custom units", "+to_meter=2.5", 2.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srid, err := RegisterProj4Defn("+proj=utm +zone=31 +datum=WGS84 " + tc.units)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ProjectedMetersPerUnit(srid)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-tc.want) > tc.want*1e-12 {
				t.Errorf("meters per unit = %v, want %v", got, tc.want)
			}
		})
	}
	RegisterBuiltinProj4SRIDs()
	for _, srid := range []uint64{3857, 3395, 4087, 32631, 32737, 28407} {
		got, err := ProjectedMetersPerUnit(srid)
		if err != nil || got != 1 {
			t.Errorf("SRID %d: factor %v, error %v; want 1, nil", srid, got, err)
		}
	}
	for _, srid := range []uint64{0, 4326, 999999} {
		if _, err := ProjectedMetersPerUnit(srid); err == nil {
			t.Errorf("SRID %d should reject unknown or non-linear units", srid)
		}
	}
}

func TestGeographicMetersPerLongitudeDegree(t *testing.T) {
	// Independent 50-digit decimal reference values, parameterized by a and
	// inverse flattening rather than the engine's derived eccentricity.
	for _, tc := range []struct {
		name, defn     string
		latitude, want float64
	}{
		{"WGS84 equator", "", 0, 111319.49079327357},
		{"WGS84 north", "", 60, 55800.00157243613},
		{"WGS84 south", "", -60, 55800.00157243613},
		{"WGS84 near pole", "", 89.999, 1.9494276978610467},
		{"registered WGS84", "+proj=longlat +datum=WGS84", 45, 78846.83509397811},
		{"Bessel", "+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93", 60, 55793.108216124725},
		{"custom ellipsoid", "+proj=longlat +a=6378200 +rf=298.3 +towgs84=41,-107.6,-93", 60, 55800.53258131389},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srid := uint64(4326)
			if tc.defn != "" {
				var err error
				srid, err = RegisterProj4Defn(tc.defn)
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := GeographicMetersPerLongitudeDegree(srid, tc.latitude)
			if err != nil {
				t.Fatal(err)
			}
			if math.IsNaN(got) || math.Abs(got-tc.want) > tc.want*1e-10 {
				t.Errorf("meters per longitude degree = %.12f, want %.12f", got, tc.want)
			}
		})
	}
	for _, latitude := range []float64{-90, 90, -91, 91, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := GeographicMetersPerLongitudeDegree(4326, latitude); err == nil {
			t.Errorf("accepted singular or invalid latitude %v", latitude)
		}
	}
	for _, srid := range []uint64{0, 3857, 32631, 999999} {
		if _, err := GeographicMetersPerLongitudeDegree(srid, 45); err == nil {
			t.Errorf("accepted non-geographic SRID %d", srid)
		}
	}
}

func TestGeographicScaleRegistrationReplacement(t *testing.T) {
	const srid = 910046
	if err := RegisterProj4SRID(srid, "+proj=longlat +ellps=bessel +towgs84=0,0,0"); err != nil {
		t.Fatal(err)
	}
	if _, err := GeographicMetersPerLongitudeDegree(srid, 60); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProj4SRID(srid, "+proj=longlat +datum=WGS84"); err != nil {
		t.Fatal(err)
	}
	got, err := GeographicMetersPerLongitudeDegree(srid, 60)
	if err != nil || math.Abs(got-55800.00157243613) > 1e-7 {
		t.Fatalf("ellipsoid retained after replacement: %v, %v", got, err)
	}
	if err := RegisterProj4SRID(srid, "+proj=utm +zone=31 +datum=WGS84"); err != nil {
		t.Fatal(err)
	}
	if _, err := GeographicMetersPerLongitudeDegree(srid, 60); err == nil {
		t.Fatal("projected replacement treated as geographic")
	}
}
