package mysql

import (
	"fmt"
	"strconv"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func (l Layer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	if l.feature == nil || l.featureError != nil || l.feature.crsProjection == nil {
		return provider.FeatureCRSDefinition{}, featureUnsupported("source CRS proof unavailable")
	}
	return l.feature.crs, nil
}
func (f *featureProfile) freezeFeatureCRS() {
	f.crs, f.crsProjection = provider.FeatureCRSDefinition{}, nil
	// Native export interprets database SRS axes. Numeric labels do not prove
	// that database definition, even when a source SRID is configured explicitly.
	if f.native || !f.crsDeclared {
		return
	}
	definition, ok := basic.EffectiveProj4Definition(f.srid)
	if !ok {
		return
	}
	projection, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		return
	}
	proof := provider.FeatureCRSDefinition{HorizontalSRID: f.srid, Definition: definition, Spatial: f.spatial}
	if !basic.IsSyntheticSRID(f.srid) && projection.CanonicalSRID() == f.srid {
		proof.CanonicalAuthority = "EPSG"
		proof.CanonicalCode = strconv.FormatUint(f.srid, 10)
	}
	if err := proof.Validate(); err != nil {
		return
	}
	f.crs, f.crsProjection = proof, projection
}

func featureCRSUnsupported() error {
	return fmt.Errorf("mysql frozen source CRS unavailable: %w", provider.ErrUnsupported)
}
