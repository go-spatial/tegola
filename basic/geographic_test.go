package basic

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
)

func TestRegisterGeographicDefinition(t *testing.T) {
	const defn = "+proj=longlat +datum=WGS84 +no_defs +type=crs"
	code, err := RegisterProj4Defn(defn)
	if err != nil {
		t.Fatal(err)
	}
	if repeated, err := RegisterProj4Defn(defn); err != nil || repeated != code {
		t.Fatalf("registration is not stable: %d, %v", repeated, err)
	}
	const explicit = 910042
	if err := RegisterProj4SRID(explicit, defn); err != nil {
		t.Fatal(err)
	}
	for _, srid := range []uint64{4326, code, explicit} {
		if !IsGeographicSRID(srid) {
			t.Fatalf("SRID %d is not recognized as geographic", srid)
		}
		if _, err := ProjectedMetersPerUnit(srid); err == nil {
			t.Fatalf("SRID %d incorrectly has linear units", srid)
		}
		for _, point := range []geom.Point{{37.6, 55.7}, {-73.9, 40.7}, {151, -33}, {0, 0}} {
			want, err := ToWebMercator(4326, point)
			if err != nil {
				t.Fatal(err)
			}
			projected, err := ToWebMercator(srid, point)
			if err != nil || projected != want {
				t.Fatalf("SRID %d: projection %v, want %v, error %v", srid, projected, want, err)
			}
			back, err := FromWebMercator(srid, projected)
			if err != nil {
				t.Fatal(err)
			}
			got := back.(geom.Point)
			if math.Abs(got[0]-point[0]) > 1e-9 || math.Abs(got[1]-point[1]) > 1e-9 {
				t.Fatalf("SRID %d: round trip %v, want %v", srid, got, point)
			}
		}
	}
	if err := RegisterProj4SRID(explicit, "+proj=utm +zone=31 +datum=WGS84"); err != nil {
		t.Fatal(err)
	}
	if IsGeographicSRID(explicit) || IsGeographicSRID(3857) || IsGeographicSRID(0) {
		t.Fatal("projected or unknown SRID classified as geographic")
	}
	projected, err := FromWebMercator(explicit, geom.Point{0, 0})
	if err != nil || math.Abs(projected.(geom.Point)[0]-166021.443) > 0.01 {
		t.Fatalf("replaced geographic conversion retained stale cache: %v, %v", projected, err)
	}
	if err := RegisterProj4SRID(explicit, defn); err != nil {
		t.Fatal(err)
	}
	geographic, err := FromWebMercator(explicit, geom.Point{0, 0})
	if err != nil || math.Abs(geographic.(geom.Point)[0]) > 1e-9 || math.Abs(geographic.(geom.Point)[1]) > 1e-9 {
		t.Fatalf("replaced projected conversion retained stale cache: %v, %v", geographic, err)
	}
}

func TestRegisterUnsupportedGeographicDefinition(t *testing.T) {
	for _, defn := range []string{
		"+proj=longlat +datum=NAD27",
		"+proj=longlat +datum=WGS84 +units=m",
		"+proj=longlat +datum=WGS84 +towgs84=1,0",
		"+proj=longlat +datum=WGS84 +towgs84=NaN,0,0",
		"+proj=longlat +datum=WGS84 +pm=paris",
	} {
		if _, err := RegisterProj4Defn(defn); err == nil {
			t.Errorf("accepted unsupported geographic definition %q", defn)
		}
		if err := RegisterProj4SRID(910043, defn); err == nil {
			t.Errorf("accepted unsupported explicit geographic definition %q", defn)
		}
	}
}

// These independent PROJ 9.5.1 reference values cover the complete crs_defn
// registration -> geographic datum conversion -> Web Mercator route, and its
// reverse. The reverse input represents WGS84 (37.6, 55.7), not the shifted point.
func TestRegisterGeographicDatumDefinition(t *testing.T) {
	for _, tc := range []struct {
		name, defn                   string
		webMercator, fromWebMercator geom.Point
	}{
		{"three_parameters", "+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93 +no_defs",
			geom.Point{4185417.6034322004, 7498990.087942868}, geom.Point{37.60175371636222, 55.69966788487392}},
		{"seven_parameters", "+proj=longlat +ellps=krass +towgs84=23.92,-141.27,-80.9,0,-0.35,-0.82,-0.12 +no_defs",
			geom.Point{4185373.1079426813, 7498961.8039867915}, geom.Point{37.602153621265074, 55.699811015165196}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srid, err := RegisterProj4Defn(tc.defn)
			if err != nil {
				t.Fatal(err)
			}
			if !IsGeographicSRID(srid) {
				t.Fatal("registered CRS is not geographic")
			}
			projected, err := ToWebMercator(srid, geom.Point{37.6, 55.7})
			if err != nil {
				t.Fatal(err)
			}
			got := projected.(geom.Point)
			for i, want := range tc.webMercator {
				if math.IsNaN(got[i]) || math.Abs(got[i]-want) > 0.001 {
					t.Errorf("Web Mercator coordinate %d = %.9f, PROJ reference %.9f", i, got[i], want)
				}
			}
			unprojected, err := FromWebMercator(srid, geom.Point{4185612.8538270863, 7498924.477653493})
			if err != nil {
				t.Fatal(err)
			}
			got = unprojected.(geom.Point)
			for i, want := range tc.fromWebMercator {
				if math.IsNaN(got[i]) || math.Abs(got[i]-want) > 2e-9 {
					t.Errorf("geographic coordinate %d = %.12f, PROJ reference %.12f", i, got[i], want)
				}
			}
		})
	}
}
