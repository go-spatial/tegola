package crsconfig

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
)

// A layer retains the CRS value returned during registration. A later layer
// must not silently change what that already-stored identifier means.
func TestResolveLayerCollisionPreservesPreviouslyResolvedCRS(t *testing.T) {
	const first = "+proj=merc +a=6370997 +b=6370997 +lon_0=0 +lat_0=0 +x_0=00092200 +units=m +no_defs"
	const second = "+proj=merc +a=6370997 +b=6370997 +lon_0=0 +lat_0=0 +x_0=00065141 +units=m +no_defs"
	layer, err := ResolveLayer(dict.Dict{KeyCRSDefn: first}, 3857)
	if err != nil {
		t.Fatal(err)
	}
	stored := proj.EPSGCode(layer.SRID)
	before, err := proj.Convert(stored, []float64{10, 50})
	if err != nil {
		t.Fatal(err)
	}
	inverse, err := proj.Inverse(stored, before)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLayer(dict.Dict{KeyCRSDefn: second}, 3857); err == nil || !strings.Contains(err.Error(), "synthetic SRID collision") {
		t.Fatalf("expected layer collision error, got %v", err)
	}
	after, err := proj.Convert(stored, []float64{10, 50})
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("stored layer CRS changed: %v %v", after, err)
	}
	inverseAfter, err := proj.Inverse(stored, before)
	if err != nil || !reflect.DeepEqual(inverse, inverseAfter) {
		t.Fatalf("stored inverse CRS changed: %v %v", inverseAfter, err)
	}
	if code, ok := basic.Proj4DefnSRID(first); !ok || int(code) != layer.SRID {
		t.Fatalf("registry no longer matches stored layer CRS: %v %v", code, ok)
	}
	if _, ok := basic.Proj4DefnSRID(second); ok {
		t.Fatal("rejected layer acquired a registry entry")
	}
}
