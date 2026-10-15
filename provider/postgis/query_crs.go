package postgis

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

type featureSRSTuple struct {
	Authority  string
	Code       int64
	WKT, Proj4 string
}

func readFeatureSRSTuple(ctx context.Context, db featureCatalogReader, f *featureProfile) (featureSRSTuple, error) {
	var tuple featureSRSTuple
	statement := "SELECT CASE WHEN pg_catalog.octet_length(auth_name)<=64 THEN auth_name ELSE NULL END,auth_srid," +
		"CASE WHEN pg_catalog.octet_length(srtext)<=65536 THEN srtext ELSE NULL END," +
		"CASE WHEN pg_catalog.octet_length(proj4text)<=65536 THEN proj4text ELSE NULL END FROM " +
		pgQuoteIdent(f.postgisSchema) + ".spatial_ref_sys WHERE srid=$1"
	err := db.QueryRow(ctx, statement, f.srid).Scan(&tuple.Authority, &tuple.Code, &tuple.WKT, &tuple.Proj4)
	return tuple, err
}

func (tuple featureSRSTuple) canonical(srid uint64) bool {
	if tuple.Authority != "EPSG" || tuple.Code <= 0 || uint64(tuple.Code) != srid {
		return false
	}
	for _, text := range []string{tuple.WKT, tuple.Proj4} {
		if len(text) > provider.MaxFeatureCRSDefinitionBytes || !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
			return false
		}
	}
	expected, ok := featureCanonicalSRSTuples[srid]
	if !ok {
		return false
	}
	text := fmt.Sprintf("%d\x00%s\x00%d\x00%s\x00%s", srid, tuple.Authority, tuple.Code, tuple.WKT, tuple.Proj4)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) == expected
}

func (p *Provider) freezeFeatureCRS(ctx context.Context, f *featureProfile) {
	if f.format != "" && !f.crsExplicit {
		return
	}
	definition, known := basic.EffectiveProj4Definition(f.srid)
	if !known {
		return
	}
	projection, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		return
	}
	proof := provider.FeatureCRSDefinition{HorizontalSRID: f.srid, Definition: definition, Spatial: f.spatial}
	if f.format == "" {
		tuple, err := readFeatureSRSTuple(ctx, p.pool, f)
		if err != nil || !tuple.canonical(f.srid) || projection.CanonicalSRID() != f.srid {
			return
		}
		f.nativeCRSTuple = tuple
		proof.CanonicalAuthority, proof.CanonicalCode = "EPSG", strconv.FormatUint(f.srid, 10)
	} else if canonical, ok := crsconfig.CanonicalFeatureDefinition(f.srid); ok && definition == canonical {
		proof.CanonicalAuthority, proof.CanonicalCode = "EPSG", strconv.FormatUint(f.srid, 10)
	}
	f.crs, f.projection = proof, projection
}

func (l Layer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.FeatureCRSDefinition{}, err
	}
	if l.feature.projection == nil {
		return provider.FeatureCRSDefinition{}, featureUnsupported("immutable source CRS proof unavailable")
	}
	return l.feature.crs, nil
}

func (f *featureProfile) prepareFeatureCRS(q provider.FeatureQuery) (*featureProfile, error) {
	if q.BoundsCRSDefinition == "" {
		return f, nil
	}
	if f.projection == nil {
		return nil, featureUnsupported("immutable source CRS proof unavailable")
	}
	target, err := crsconfig.NewFeatureProjection(q.BoundsCRSDefinition)
	if err != nil {
		return nil, featureUnsupported("immutable query CRS profile unsupported")
	}
	copy := *f
	copy.queryProjection = target
	return &copy, nil
}
