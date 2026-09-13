package basic_test

import (
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
)

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
