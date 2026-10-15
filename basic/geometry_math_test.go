package basic_test

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
)

// TestWebMercator3395 verifies EPSG:3395 (WGS84 World Mercator, ellipsoidal)
// reprojects to and from Web Mercator (3857). At mid latitudes the two
// Mercator variants differ by several km, which is exactly the distortion
// this path fixes; a round trip must return the original point.
func TestWebMercator3395(t *testing.T) {
	// Chelyabinsk-ish coordinates used in live testing: lon 61.4, lat 55.1.
	ll := []float64{61.4, 55.1}

	// build the 3395 meter representation of that lon/lat via proj directly
	m, err := proj.Convert(proj.EPSG3395, ll)
	if err != nil {
		t.Fatalf("proj.Convert(3395): %v", err)
	}
	p3395 := geom.Point{m[0], m[1]}

	// 3395 -> 3857: result must be valid Web Mercator meters and must match
	// the spherical (4326) path — both routes resolve to the same lon/lat.
	// The ellipsoidal/spherical difference shows up in the 3395 meters
	// themselves (vs 3857), not in the 3857 output.
	web, err := basic.ToWebMercator(3395, p3395)
	if err != nil {
		t.Fatalf("ToWebMercator(3395): %v", err)
	}
	wp, ok := web.(geom.Point)
	if !ok {
		t.Fatalf("expected geom.Point, got %T", web)
	}
	if math.Abs(wp[0]) > 20037508.43 || math.Abs(wp[1]) > 20048966.1 {
		t.Errorf("point out of Web Mercator bounds: %v", wp)
	}

	spherical, err := basic.ToWebMercator(tegola.WGS84, geom.Point{ll[0], ll[1]})
	if err != nil {
		t.Fatalf("ToWebMercator(4326): %v", err)
	}
	sp := spherical.(geom.Point)
	if math.Abs(wp[0]-sp[0]) > 1 || math.Abs(wp[1]-sp[1]) > 1 {
		t.Errorf("3395->3857 (%v) should match 4326->3857 (%v)", wp, sp)
	}

	// at 55 deg latitude the ellipsoidal (3395) and spherical (3857) meter
	// values of the same location differ by km — this is exactly the
	// distortion that transparent 3395 support fixes.
	if math.Abs(p3395[1]-sp[1]) < 1000 {
		t.Errorf("expected 3395 and 3857 meter Y to differ by km, got %v vs %v", p3395[1], sp[1])
	}

	// round trip 3395 -> 3857 -> 3395
	back, err := basic.FromWebMercator(3395, wp)
	if err != nil {
		t.Fatalf("FromWebMercator(3395): %v", err)
	}
	bp := back.(geom.Point)
	if math.Abs(bp[0]-p3395[0]) > 1 || math.Abs(bp[1]-p3395[1]) > 1 {
		t.Errorf("round trip mismatch: %v -> %v", p3395, bp)
	}

	// unknown SRID still errors
	if _, err := basic.ToWebMercator(99999, p3395); err == nil {
		t.Error("expected error for unknown SRID 99999, got nil")
	}
}

func TestFromWebMercatorExtentUsesAllCorners(t *testing.T) {
	webExtent := geom.NewExtent(
		[2]float64{-1000000, 4000000},
		[2]float64{2000000, 8000000},
	)

	got, err := basic.FromWebMercatorExtent(tegola.WGS84, webExtent)
	if err != nil {
		t.Fatalf("FromWebMercatorExtent(4326): %v", err)
	}

	corners := webExtent.Vertices()
	for _, corner := range corners {
		source, err := basic.FromWebMercator(tegola.WGS84, geom.Point{corner[0], corner[1]})
		if err != nil {
			t.Fatalf("FromWebMercator(4326): %v", err)
		}
		point := source.(geom.Point)
		if point[0] < got.MinX() || point[0] > got.MaxX() || point[1] < got.MinY() || point[1] > got.MaxY() {
			t.Fatalf("converted corner %v is outside extent %v", point, got)
		}
	}
}

// TestWebMercatorBuiltinEPSG verifies that SRIDs from the built-in table
// (UTM, Gauss-Kruger, mercator aliases) convert to and from Web Mercator
// without any explicit registration beyond RegisterBuiltinProj4SRIDs, and
// that a round trip returns the original point.
func TestWebMercatorBuiltinEPSG(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()

	// Chelyabinsk falls in UTM zone 41N (EPSG:32641).
	ll := []float64{61.4, 55.1}
	utm, err := proj.Convert(proj.EPSGCode(32641), ll)
	if err != nil {
		t.Fatalf("proj.Convert(32641) not registered: %v", err)
	}
	pUTM := geom.Point{utm[0], utm[1]}

	web, err := basic.ToWebMercator(32641, pUTM)
	if err != nil {
		t.Fatalf("ToWebMercator(32641): %v", err)
	}
	wp := web.(geom.Point)

	spherical, err := basic.ToWebMercator(tegola.WGS84, geom.Point{ll[0], ll[1]})
	if err != nil {
		t.Fatalf("ToWebMercator(4326): %v", err)
	}
	sp := spherical.(geom.Point)
	if math.Abs(wp[0]-sp[0]) > 1 || math.Abs(wp[1]-sp[1]) > 1 {
		t.Errorf("32641->3857 (%v) should match 4326->3857 (%v)", wp, sp)
	}

	back, err := basic.FromWebMercator(32641, wp)
	if err != nil {
		t.Fatalf("FromWebMercator(32641): %v", err)
	}
	bp := back.(geom.Point)
	if math.Abs(bp[0]-pUTM[0]) > 1 || math.Abs(bp[1]-pUTM[1]) > 1 {
		t.Errorf("round trip mismatch: %v -> %v", pUTM, bp)
	}

	// Pulkovo 1942 / Gauss-Kruger zone 8 (EPSG:28408), sanity round trip.
	gk, err := proj.Convert(proj.EPSGCode(28408), ll)
	if err != nil {
		t.Fatalf("proj.Convert(28408) not registered: %v", err)
	}
	gkWeb, err := basic.ToWebMercator(28408, geom.Point{gk[0], gk[1]})
	if err != nil {
		t.Fatalf("ToWebMercator(28408): %v", err)
	}
	gkBack, err := basic.FromWebMercator(28408, gkWeb)
	if err != nil {
		t.Fatalf("FromWebMercator(28408): %v", err)
	}
	gkb := gkBack.(geom.Point)
	if math.Abs(gkb[0]-gk[0]) > 1 || math.Abs(gkb[1]-gk[1]) > 1 {
		t.Errorf("28408 round trip mismatch: %v -> %v", geom.Point{gk[0], gk[1]}, gkb)
	}

	// Pulkovo 1995 GK zone (EPSG:2463..2491 family), sanity round trip.
	p95, err := proj.Convert(proj.EPSGCode(2463), ll)
	if err != nil {
		t.Fatalf("proj.Convert(2463) not registered: %v", err)
	}
	p95Web, err := basic.ToWebMercator(2463, geom.Point{p95[0], p95[1]})
	if err != nil {
		t.Fatalf("ToWebMercator(2463): %v", err)
	}
	p95Back, err := basic.FromWebMercator(2463, p95Web)
	if err != nil {
		t.Fatalf("FromWebMercator(2463): %v", err)
	}
	p95b := p95Back.(geom.Point)
	if math.Abs(p95b[0]-p95[0]) > 1 || math.Abs(p95b[1]-p95[1]) > 1 {
		t.Errorf("2463 round trip mismatch: %v -> %v", geom.Point{p95[0], p95[1]}, p95b)
	}

	// 3857 aliases (3785, 900913) must be identical to Web Mercator itself.
	for _, alias := range []uint64{3785, 900913, 53004} {
		_, err := proj.Convert(proj.EPSGCode(alias), ll)
		if err != nil {
			t.Fatalf("proj.Convert(%v) not registered: %v", alias, err)
		}
		if !basic.IsBuiltinProj4SRID(alias) {
			t.Errorf("alias %v should be in the built-in table", alias)
		}
	}

	// unknown SRID still errors through the generic path
	if _, err := basic.ToWebMercator(99999, pUTM); err == nil {
		t.Error("expected error for unknown SRID 99999, got nil")
	}
}

// TestRegisterProj4SRID verifies custom EPSG registration through a PROJ.4
// string makes the SRID convertible, is idempotent, and rejects garbage.
func TestRegisterProj4SRID(t *testing.T) {
	// EPSG:3844 (ST70 / Stereographic 70) uses +proj=stere which the vendored
	// proj does not implement; use an aea (Albers) definition instead —
	// also unregistered, so it exercises the same custom path.
	const def = "+proj=aea +lat_1=46 +lat_2=52 +lat_0=49 +lon_0=25 +x_0=500000 +y_0=500000 +ellps=krass +units=m +no_defs"
	if err := basic.RegisterProj4SRID(3844, def); err != nil {
		t.Fatalf("RegisterProj4SRID(3844): %v", err)
	}
	// idempotent re-registration must not fail
	if err := basic.RegisterProj4SRID(3844, def); err != nil {
		t.Fatalf("re-register 3844: %v", err)
	}

	ll := []float64{25.0, 46.0}
	m, err := proj.Convert(proj.EPSGCode(3844), ll)
	if err != nil {
		t.Fatalf("proj.Convert(3844): %v", err)
	}
	p := geom.Point{m[0], m[1]}

	web, err := basic.ToWebMercator(3844, p)
	if err != nil {
		t.Fatalf("ToWebMercator(3844): %v", err)
	}
	back, err := basic.FromWebMercator(3844, web)
	if err != nil {
		t.Fatalf("FromWebMercator(3844): %v", err)
	}
	bp := back.(geom.Point)
	if math.Abs(bp[0]-p[0]) > 1 || math.Abs(bp[1]-p[1]) > 1 {
		t.Errorf("3844 round trip mismatch: %v -> %v", p, bp)
	}

	// unsupported operations must be rejected
	if err := basic.RegisterProj4SRID(3845, "+proj=stere +lat_0=46"); err == nil {
		t.Error("expected error for unsupported projection, got nil")
	}
	if err := basic.RegisterProj4SRID(0, def); err == nil {
		t.Error("expected error for srid 0, got nil")
	}
	if err := basic.RegisterProj4SRID(3846, "  "); err == nil {
		t.Error("expected error for empty proj4, got nil")
	}
}

// TestParseProj4Config covers the string form of the proj4 config option.
func TestParseProj4Config(t *testing.T) {
	defs, err := basic.ParseProj4Config([]string{
		"3844 = +proj=aea +lat_1=46",
		"",
		"EPSG:32601=+proj=utm +zone=1",
	})
	if err != nil {
		t.Fatalf("ParseProj4Config: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("expected 2 entries, got %v: %v", len(defs), defs)
	}
	if defs[3844] != "+proj=aea +lat_1=46" {
		t.Errorf("entry 3844 wrong: %q", defs[3844])
	}
	if defs[32601] != "+proj=utm +zone=1" {
		t.Errorf("entry 32601 wrong: %q", defs[32601])
	}

	if _, err := basic.ParseProj4Config([]string{"nonsense"}); err == nil {
		t.Error("expected error for entry without '=', got nil")
	}
	if _, err := basic.ParseProj4Config([]string{"abc=+proj=merc"}); err == nil {
		t.Error("expected error for non-numeric srid, got nil")
	}
}

// TestToWebMercatorCollection ensures that a geom.Collection (as produced by
// GPKG GEOMETRYCOLLECTION rows, which are common in data converted from CAD
// formats such as DWG) can be reprojected without erroring out. Previously
// ApplyToPoints/CloneGeometry did not have a case for geom.Collection, so any
// feature with this geometry type would fail to reproject and abort the
// whole tile.
func TestToWebMercatorCollection(t *testing.T) {
	collection := geom.Collection{
		geom.Point{10, 20},
		geom.LineString{{10, 20}, {30, 40}},
	}

	got, err := basic.ToWebMercator(tegola.WGS84, collection)
	if err != nil {
		t.Fatalf("unexpected error reprojecting collection: %v", err)
	}

	gotColl, ok := got.(geom.Collection)
	if !ok {
		t.Fatalf("expected geom.Collection, got %T", got)
	}
	if len(gotColl) != len(collection) {
		t.Fatalf("expected %v geometries, got %v", len(collection), len(gotColl))
	}
	if _, ok := gotColl[0].(geom.Point); !ok {
		t.Fatalf("expected first geometry to be a Point, got %T", gotColl[0])
	}
	if _, ok := gotColl[1].(geom.LineString); !ok {
		t.Fatalf("expected second geometry to be a LineString, got %T", gotColl[1])
	}
}

// TestCloneGeometryCollection ensures CloneGeometry can clone nested Collections,
// which is exercised when the SRID already matches WebMercator.
func TestCloneGeometryCollection(t *testing.T) {
	collection := geom.Collection{
		geom.Point{1, 2},
		geom.Collection{geom.Point{3, 4}},
	}

	got, err := basic.ToWebMercator(tegola.WebMercator, collection)
	if err != nil {
		t.Fatalf("unexpected error cloning collection: %v", err)
	}

	gotColl, ok := got.(geom.Collection)
	if !ok {
		t.Fatalf("expected geom.Collection, got %T", got)
	}
	if len(gotColl) != 2 {
		t.Fatalf("expected 2 geometries, got %v", len(gotColl))
	}
	nested, ok := gotColl[1].(geom.Collection)
	if !ok {
		t.Fatalf("expected nested geom.Collection, got %T", gotColl[1])
	}
	if len(nested) != 1 {
		t.Fatalf("expected 1 nested geometry, got %v", len(nested))
	}
}
