package hana

import (
	"strconv"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func freezeFeatureCRS(s *featureSource) {
	definition, known := basic.EffectiveProj4Definition(s.SRID)
	if !known {
		return
	}
	projection, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		return
	}
	proof := provider.FeatureCRSDefinition{HorizontalSRID: s.SRID, Definition: definition, Spatial: s.Spatial}
	// Native source meaning already passed the complete immutable HANA SRS
	// tuple check. An overridden client definition must agree with that meaning.
	nativeSource := false
	for _, native := range s.Catalog.Native {
		nativeSource = nativeSource || native.Name == s.physical(s.Geometry)
	}
	if nativeSource {
		if projection.CanonicalSRID() != 4326 {
			return
		}
		proof.CanonicalAuthority, proof.CanonicalCode = "EPSG", "4326"
	} else if canonical, ok := crsconfig.CanonicalFeatureDefinition(s.SRID); ok && definition == canonical {
		proof.CanonicalAuthority, proof.CanonicalCode = "EPSG", strconv.FormatUint(s.SRID, 10)
	}
	s.CRS, s.Projection = proof, projection
}

func (l Layer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.FeatureCRSDefinition{}, err
	}
	if l.feature.Projection == nil {
		return provider.FeatureCRSDefinition{}, featureUnsupported("immutable source CRS proof unavailable")
	}
	return l.feature.CRS, nil
}
