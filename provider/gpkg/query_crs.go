//go:build cgo

package gpkg

import (
	"fmt"
	"strconv"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func (l Layer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	if l.FeatureQuerySupported() != nil || l.featureCRSProjection == nil || l.featureCRSError != nil {
		return provider.FeatureCRSDefinition{}, fmt.Errorf("gpkg source CRS proof unavailable: %w", provider.ErrUnsupported)
	}
	return l.featureCRS, nil
}

func (l *Layer) freezeFeatureCRS() {
	l.featureCRS, l.featureCRSProjection, l.featureCRSError = provider.FeatureCRSDefinition{}, nil, nil
	if !l.crsConfigured {
		l.featureCRSError = fmt.Errorf("gpkg source CRS declaration is unproven: %w", provider.ErrUnsupported)
		return
	}
	srid := resolvedLayerSRID(l)
	definition, ok := basic.EffectiveProj4Definition(srid)
	if !ok {
		l.featureCRSError = fmt.Errorf("gpkg source definition unavailable: %w", provider.ErrUnsupported)
		return
	}
	projection, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		l.featureCRSError = fmt.Errorf("gpkg source definition unsupported: %w", provider.ErrUnsupported)
		return
	}
	proof := provider.FeatureCRSDefinition{HorizontalSRID: srid, Definition: definition, Spatial: l.spatialMetadata}
	if !basic.IsSyntheticSRID(srid) && projection.CanonicalSRID() == srid {
		proof.CanonicalAuthority = "EPSG"
		proof.CanonicalCode = strconv.FormatUint(srid, 10)
	}
	if err := proof.Validate(); err != nil {
		l.featureCRSError = fmt.Errorf("gpkg source proof invalid: %w", provider.ErrUnsupported)
		return
	}
	l.featureCRS, l.featureCRSProjection = proof, projection
}
