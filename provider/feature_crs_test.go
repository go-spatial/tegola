package provider

import (
	"strings"
	"testing"

	"github.com/alexeydott/geom"
)

func TestFeatureCRSProofAndAuthoritativeBoundsShape(t *testing.T) {
	proof := FeatureCRSDefinition{HorizontalSRID: 4326, Definition: "+proj=longlat +datum=WGS84", Spatial: SpatialMetadata{Dimension: DimensionXY}}
	if err := proof.Validate(); err != nil {
		t.Fatal(err)
	}
	query := FeatureQuery{Limit: 1, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326, BoundsCRSDefinition: proof.Definition}
	if err := query.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, definition := range []string{" ", "x\x00", string([]byte{0xff}), strings.Repeat("x", MaxFeatureCRSDefinitionBytes+1)} {
		query.BoundsCRSDefinition = definition
		if err := query.Validate(); err == nil {
			t.Fatal("invalid definition accepted")
		}
	}
	query.BoundsCRSDefinition = proof.Definition
	query.Bounds = nil
	query.BoundsSRID = 0
	if err := query.Validate(); err == nil {
		t.Fatal("orphan definition accepted")
	}
	proof.CanonicalAuthority = "EPSG"
	if err := proof.Validate(); err == nil {
		t.Fatal("partial authority accepted")
	}
	proof.CanonicalCode = "4326"
	if err := proof.Validate(); err != nil {
		t.Fatal(err)
	}
}
