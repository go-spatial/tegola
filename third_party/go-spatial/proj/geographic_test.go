package proj

import (
	"math"
	"reflect"
	"testing"
)

func TestGeographicConversion(t *testing.T) {
	const code EPSGCode = 330000001
	defer RemoveCustomProjection(code)
	for _, defn := range []string{
		"+proj=longlat +datum=WGS84 +no_defs",
		"+proj=latlong +ellps=WGS84 +towgs84=0,0,0 +type=crs",
		"+proj=lonlat +datum=WGS84 +ellps=WGS84 +pm=greenwich +axis=enu +units=degrees",
		"+proj=latlon +datum=WGS84 +towgs84=0,0,0,0,0,0,0",
	} {
		CustomProjection(code, defn)
		input := []float64{37.6, 55.7, -120, -45, 180, 90}
		for _, convert := range []func(EPSGCode, []float64) ([]float64, error){Convert, Inverse} {
			got, err := convert(code, input)
			if err != nil || !reflect.DeepEqual(got, input) {
				t.Fatalf("%s: got %v, %v", defn, got, err)
			}
			got[0] = 0
			if input[0] != 37.6 {
				t.Fatal("conversion aliases input")
			}
			for _, invalid := range [][]float64{{1}, {1, 91}, {math.NaN(), 1}, {1, math.Inf(1)}} {
				if _, err := convert(code, invalid); err == nil {
					t.Fatalf("accepted invalid coordinates %v", invalid)
				}
			}
		}
	}
}

func TestGeographicRejectsUnsupportedSemantics(t *testing.T) {
	const code EPSGCode = 330000002
	defer RemoveCustomProjection(code)
	for _, defn := range []string{
		"+proj=longlat",
		"+proj=longlat +datum=NAD27",
		"+proj=longlat +datum=WGS84 +ellps=clrk66",
		"+proj=longlat +datum=WGS84 +a=6371000",
		"+proj=longlat +datum=WGS84 +towgs84=1,0",
		"+proj=longlat +datum=WGS84 +towgs84=1,0,0,0",
		"+proj=longlat +datum=WGS84 +towgs84=Inf,0,0",
		"+proj=longlat +datum=WGS84 +towgs84=0,0,0,0,0,0,-1000000",
		"+proj=longlat +datum=WGS84 +towgs84=NaN,0,0",
		"+proj=longlat +datum=WGS84 +nadgrids=@null",
		"+proj=longlat +datum=WGS84 +units=rad",
		"+proj=longlat +datum=WGS84 +units=m",
		"+proj=longlat +datum=WGS84 +to_meter=1",
		"+proj=longlat +datum=WGS84 +pm=paris",
		"+proj=longlat +datum=WGS84 +axis=neu",
		"+proj=longlat +datum=WGS84 +lon_0=10",
		"+proj=longlat +datum=WGS84 +proj=merc",
		"+proj=longlat +datum=WGS84 +datum=NAD27",
	} {
		if IsGeographicDefinition(defn) {
			t.Errorf("classified unsupported definition %s", defn)
		}
		CustomProjection(code, defn)
		if _, err := Convert(code, []float64{10, 50}); err == nil {
			t.Errorf("accepted unsupported definition %s", defn)
		}
		if _, err := Inverse(code, []float64{10, 50}); err == nil {
			t.Errorf("inverse accepted unsupported definition %s", defn)
		}
	}
}

// Reference coordinates were generated with pyproj 3.7.2 / PROJ 9.5.1,
// Transformer.from_crs(..., always_xy=True). Each direction starts at zero
// ellipsoidal height, as does this two-dimensional API. Independent expectations
// in both directions detect errors that a round trip could otherwise conceal.
func TestGeographicDatumReference(t *testing.T) {
	const code EPSGCode = 330000003
	defer RemoveCustomProjection(code)
	input := []float64{37.6, 55.7, -73.9, 40.7, 151, -33}
	for _, tc := range []struct {
		name, defn         string
		toWGS84, fromWGS84 []float64
	}{
		{"three_parameters", "+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93 +no_defs",
			[]float64{37.598246035860434, 55.70033213366124, -73.89988695789151, 40.699282924421865, 151.00079442894847, -33.00168273757353},
			[]float64{37.60175371636222, 55.69966788487392, -73.90011303387108, 40.700716998828185, 150.99920569168285, -32.99831747430245}},
		{"seven_parameters", "+proj=longlat +ellps=krass +towgs84=23.92,-141.27,-80.9,0,-0.35,-0.82,-0.12 +no_defs",
			[]float64{37.597846326077345, 55.70018895442415, -73.90049918141113, 40.69866281790788, 151.00093985641197, -33.001158371922934},
			[]float64{37.60215362126508, 55.69981101516519, -73.89950079166913, 40.70133722631409, 150.99906015986528, -32.99884161230262}},
		{"custom_ellipsoid", "+proj=longlat +a=6378200 +rf=298.3 +towgs84=41,-107.6,-93",
			[]float64{37.598246268091366, 55.69979927750731}, []float64{37.60175371636222, 55.70020069354171}},
		{"zero_shift_different_ellipsoid", "+proj=longlat +ellps=bessel +towgs84=0,0,0",
			[]float64{37.6, 55.700556783240685}, []float64{37.6, 55.699443273120835}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !IsGeographicDefinition(tc.defn) {
				t.Fatal("supported datum definition not classified as geographic")
			}
			CustomProjection(code, tc.defn)
			for _, direction := range []struct {
				name    string
				convert func(EPSGCode, []float64) ([]float64, error)
				want    []float64
			}{{"to_WGS84", Inverse, tc.toWGS84}, {"from_WGS84", Convert, tc.fromWGS84}} {
				t.Run(direction.name, func(t *testing.T) {
					source := append([]float64(nil), input[:len(direction.want)]...)
					got, err := direction.convert(code, source)
					if err != nil {
						t.Fatal(err)
					}
					for i, want := range direction.want {
						if math.IsNaN(got[i]) || math.Abs(got[i]-want) > 2e-9 {
							t.Errorf("coordinate %d = %.12f, PROJ reference %.12f", i, got[i], want)
						}
					}
					if !reflect.DeepEqual(source, input[:len(direction.want)]) {
						t.Fatal("conversion modified input")
					}
					for _, invalid := range [][]float64{{1}, {1, -91}, {math.NaN(), 1}, {1, math.Inf(-1)}} {
						if _, err := direction.convert(code, invalid); err == nil {
							t.Fatalf("accepted invalid coordinates %v", invalid)
						}
					}
				})
			}
		})
	}
}

func TestGeographicDatumPoles(t *testing.T) {
	const code EPSGCode = 330000004
	defer RemoveCustomProjection(code)
	CustomProjection(code, "+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93")
	// PROJ 9.5.1, zero height at both poles. Longitude changes substantially
	// because the translation moves the point off the source rotation axis.
	input := []float64{37.6, 90, 37.6, -90}
	for _, tc := range []struct {
		convert func(EPSGCode, []float64) ([]float64, error)
		want    []float64
	}{
		{Inverse, []float64{-69.14111347210704, 89.99896896430798, -69.14111347210704, -89.99896899427706}},
		{Convert, []float64{110.85888652751954, 89.9989690812622, 110.85888652751954, -89.99896905129816}},
	} {
		got, err := tc.convert(code, input)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range tc.want {
			if math.IsNaN(got[i]) || math.Abs(got[i]-want) > 2e-9 {
				t.Errorf("coordinate %d = %.12f, PROJ reference %.12f", i, got[i], want)
			}
		}
	}
}

func TestGeographicNamedDatum(t *testing.T) {
	const code EPSGCode = 330000005
	defer RemoveCustomProjection(code)
	const defn = "+proj=longlat +datum=GGRS87"
	if !IsGeographicDefinition(defn) {
		t.Fatal("named geographic datum not recognized")
	}
	CustomProjection(code, defn)
	// PROJ 9.5.1 GGRS87 to WGS84 (1), with ballpark transformations disabled.
	for _, tc := range []struct {
		convert func(EPSGCode, []float64) ([]float64, error)
		want    []float64
	}{
		{Inverse, []float64{23.701693950386385, 37.98259931410117}},
		{Convert, []float64{23.698306152551783, 37.97740063484464}},
	} {
		got, err := tc.convert(code, []float64{23.7, 37.98})
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range tc.want {
			if math.IsNaN(got[i]) || math.Abs(got[i]-want) > 2e-9 {
				t.Errorf("coordinate %d = %.12f, PROJ reference %.12f", i, got[i], want)
			}
		}
	}
}
