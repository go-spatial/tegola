package hana

import (
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestFeatureCRSAuthoritativeFrame(t *testing.T) {
	s := &featureSource{SRID: 4326, Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	freezeFeatureCRS(s)
	if s.Projection == nil || s.CRS.CanonicalCode != "4326" {
		t.Fatal("shipped explicit source proof unavailable")
	}
	definition, _ := crsconfig.CanonicalFeatureDefinition(3857)
	q := provider.FeatureQuery{BoundsSRID: 4326, BoundsCRSDefinition: definition, Bounds: []geom.Extent{{1100000, 2200000, 1120000, 2300000}}}
	spatial, err := newSpatialQuery(&Layer{feature: s}, q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := spatial.matches(geom.Point{10, 20}, 4326, q)
	if err != nil || !matched {
		t.Fatalf("same numeric label ignored authoritative definition: %v %v", matched, err)
	}
	if _, err := newSpatialQuery(&Layer{feature: &featureSource{}}, q); err == nil {
		t.Fatal("missing frozen source proof admitted")
	}
	q.BoundsCRSDefinition = "+proj=aea +datum=WGS84"
	if _, err := newSpatialQuery(&Layer{feature: s}, q); err == nil {
		t.Fatal("unproved target admitted")
	}
	copy := s.CRS
	copy.Definition = "changed"
	if s.CRS.Definition == copy.Definition {
		t.Fatal("metadata retained caller mutation")
	}
}
