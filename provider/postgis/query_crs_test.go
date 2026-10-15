package postgis

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	"github.com/jackc/pgx/v5"
)

func TestFeatureCRSAuthoritativeFrame(t *testing.T) {
	definition, _ := crsconfig.CanonicalFeatureDefinition(4326)
	source, _ := crsconfig.NewFeatureProjection(definition)
	f := &featureProfile{srid: 4326, projection: source, format: "", geometry: "geom", schema: "s", table: "t"}
	target, _ := crsconfig.CanonicalFeatureDefinition(3857)
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsCRSDefinition: target, Bounds: []geom.Extent{{1100000, 2200000, 1120000, 2300000}}}
	prepared, err := f.prepareFeatureCRS(q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := prepared.matchesFeatureBounds(geom.Point{10, 20}, q)
	if err != nil || !matched {
		t.Fatalf("authoritative target ignored: %v %v", matched, err)
	}
	statement, _ := prepared.chunkStatement(q, []string{"id"}, nil)
	if strings.Contains(statement, "ST_MakeEnvelope") {
		t.Fatal("numeric same-SRID enabled unsafe source index predicate")
	}
	if f.queryProjection != nil {
		t.Fatal("query mutated frozen source")
	}
	q.BoundsCRSDefinition = definition
	prepared, err = f.prepareFeatureCRS(q)
	if err != nil {
		t.Fatal(err)
	}
	statement, _ = prepared.chunkStatement(q, []string{"id"}, nil)
	if !strings.Contains(statement, "ST_MakeEnvelope") {
		t.Fatal("proven identity lost native coarse candidate predicate")
	}
	if (featureSRSTuple{Authority: "EPSG", Code: 4326, WKT: "changed", Proj4: definition}).canonical(4326) {
		t.Fatal("nominal authority proved altered definition")
	}
}

type featureCRSSnapshot struct {
	*featureTestSnapshot
	statement string
	oversize  bool
}

func (s *featureCRSSnapshot) QueryRow(ctx context.Context, statement string, args ...any) pgx.Row {
	if !strings.Contains(statement, "spatial_ref_sys") {
		return s.featureTestSnapshot.QueryRow(ctx, statement, args...)
	}
	s.statement = statement
	if s.oversize {
		return featureTestRow{values: []any{"EPSG", int64(4326), nil, "bounded"}}
	}
	return featureTestRow{values: []any{"EPSG", int64(4326), "changed definition", "changed"}}
}

func TestFeatureCRSOutputOnlySnapshotDriftAndTransferBound(t *testing.T) {
	definition, _ := crsconfig.CanonicalFeatureDefinition(4326)
	projection, _ := crsconfig.NewFeatureProjection(definition)
	for _, oversize := range []bool{false, true} {
		f := featureTestProfile()
		f.format = ""
		f.projection = projection
		f.nativeCRSTuple = featureSRSTuple{Authority: "EPSG", Code: 4326, WKT: "frozen", Proj4: definition}
		snapshot := &featureCRSSnapshot{featureTestSnapshot: &featureTestSnapshot{profile: f}, oversize: oversize}
		callbacks := 0
		_, err := executeFeatureSnapshot(context.Background(), snapshot, f, provider.FeatureQuery{Limit: 1, IDs: []uint64{1}}, nil, func(*provider.Feature) error { callbacks++; return nil })
		var data provider.FeatureDataError
		if !errors.As(err, &data) || callbacks != 0 {
			t.Fatal("output-only source drift admitted", callbacks, err)
		}
		if strings.Count(snapshot.statement, "pg_catalog.octet_length") != 3 || !strings.Contains(snapshot.statement, "ELSE NULL END") {
			t.Fatal("metadata transfer lacks source-side bounds")
		}
	}
}
