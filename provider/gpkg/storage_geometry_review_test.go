//go:build cgo

package gpkg

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/tegola/mos"
)

func TestMutationMOSBoundsMatchStoredGeometry(t *testing.T) {
	mapping := &writeMapping{geomFormat: "mos", geomType: "point", geomSRID: 3857, mosOpts: mos.Options{Precision: 2, UnitFactor: 1}}
	raw, err := wkb.EncodeBytes(geom.Point{1.2349, -2.3451})
	if err != nil {
		t.Fatal(err)
	}
	stored, _, bounds, err := encodeStorageGeometry(mapping, raw, 3857)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mos.Decode(stored, mapping.mosOpts)
	if err != nil {
		t.Fatal(err)
	}
	point, ok := decoded.(geom.Point)
	if !ok {
		t.Fatalf("decoded %T", decoded)
	}
	want := [4]float64{point[0], point[0], point[1], point[1]}
	if bounds != want {
		t.Fatalf("bounds=%v, actual stored geometry bounds=%v", bounds, want)
	}
}

func TestMutationWKTTransformedBounds(t *testing.T) {
	mapping := &writeMapping{geomFormat: "wkt", geomType: "point", geomSRID: 3857}
	raw, err := wkb.EncodeBytes(geom.Point{10, 20})
	if err != nil {
		t.Fatal(err)
	}
	_, stored, bounds, err := encodeStorageGeometry(mapping, raw, 4326)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := wkt.DecodeString(stored)
	if err != nil {
		t.Fatal(err)
	}
	point, ok := decoded.(geom.Point)
	if !ok {
		t.Fatalf("decoded %T", decoded)
	}
	if math.Abs(point[0]-1113194.9079327357) > 0.001 || math.Abs(point[1]-2273030.926987689) > 0.001 {
		t.Fatalf("transform=%v", point)
	}
	if bounds != [4]float64{point[0], point[0], point[1], point[1]} {
		t.Fatalf("stored bounds mismatch: %v vs %v", bounds, point)
	}
}
