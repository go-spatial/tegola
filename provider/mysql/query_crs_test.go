package mysql

import (
	"errors"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestFeatureCRSDefinitionOverridesSameSRID(t *testing.T) {
	f := &featureProfile{srid: 4326, crsDeclared: true, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	f.freezeFeatureCRS()
	def, _ := crsconfig.CanonicalFeatureDefinition(3857)
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsCRSDefinition: def, Bounds: []geom.Extent{{1669792, 3503549, 1669793, 3503550}}}
	s, err := newFeatureSpatialQuery(f, q)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := s.matches(geom.Point{15, 30}, 4326, q)
	if err != nil || !ok {
		t.Fatal("numeric equality hid target", ok, err)
	}
	ok, err = s.matches(geom.Point{0, 0}, 4326, q)
	if err != nil || ok {
		t.Fatal("false positive", ok, err)
	}
	layer := Layer{feature: f}
	proof, err := layer.FeatureCRSDefinition()
	if err != nil || proof.CanonicalCode != "4326" {
		t.Fatal(proof, err)
	}
	proof.Definition = "changed"
	again, _ := layer.FeatureCRSDefinition()
	if again.Definition == "changed" {
		t.Fatal("proof retained")
	}
	q.BoundsCRSDefinition = "+proj=longlat +datum=NAD83"
	if _, err := newFeatureSpatialQuery(f, q); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("unsupported target", err)
	}
	f.crsProjection = nil
	q.BoundsCRSDefinition = def
	if _, err := newFeatureSpatialQuery(f, q); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("unproved source admitted", err)
	}
	q.BoundsCRSDefinition = ""
	q.Bounds = []geom.Extent{{0, 0, 30, 40}}
	s, err = newFeatureSpatialQuery(f, q)
	if err != nil {
		t.Fatal(err)
	}
	ok, err = s.matches(geom.Point{15, 30}, 4326, q)
	if err != nil || !ok {
		t.Fatal("Core changed", ok, err)
	}
}

func TestFeatureCRSNativeOptionalUnsupportedPreservesCore(t *testing.T) {
	f := &featureProfile{srid: 4326, native: true, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	f.freezeFeatureCRS()
	layer := Layer{feature: f}
	if err := layer.FeatureQuerySupported(); err != nil {
		t.Fatal("Core disabled", err)
	}
	if _, err := layer.FeatureCRSDefinition(); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("unproved native SRS published", err)
	}
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 30, 40}}}
	s, err := newFeatureSpatialQuery(f, q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := s.matches(geom.Point{15, 30}, 4326, q)
	if err != nil || !matched {
		t.Fatal("Core changed", matched, err)
	}
}

func TestFeatureCRSRawDefaultOptionalUnsupported(t *testing.T) {
	f := &featureProfile{srid: 4326, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	f.freezeFeatureCRS()
	layer := Layer{feature: f}
	if _, err := layer.FeatureCRSDefinition(); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("default source published", err)
	}
	if err := layer.FeatureQuerySupported(); err != nil {
		t.Fatal("Core disabled", err)
	}
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 30, 40}}}
	s, err := newFeatureSpatialQuery(f, q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := s.matches(geom.Point{15, 30}, 4326, q)
	if err != nil || !matched {
		t.Fatal("Core changed", matched, err)
	}
}

func TestFeatureCRSSemanticIdentityIgnoresNumericMapping(t *testing.T) {
	f := &featureProfile{srid: 32633, crsDeclared: true, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	f.freezeFeatureCRS()
	definition, _ := crsconfig.CanonicalFeatureDefinition(32633)
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsCRSDefinition: definition, Bounds: []geom.Extent{{500000, 3318785.352581207, 500000, 3318785.352581207}}}
	s, err := newFeatureSpatialQuery(f, q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := s.matches(geom.Point{500000, 3318785.352581207}, 32633, q)
	if err != nil || !matched {
		t.Fatal("identity incurred unnecessary roundtrip", matched, err)
	}
}
