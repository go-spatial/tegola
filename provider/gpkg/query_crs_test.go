//go:build cgo

package gpkg

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestFeatureCRSDefinitionOverridesSameSRIDBeforePaging(t *testing.T) {
	p, _ := queryTestProvider(t, `CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT,MINX REAL,MAXX REAL,MINY REAL,MAXY REAL); INSERT INTO items VALUES(1,'POINT(0 0)',0,0,0,0),(2,'POINT(15 30)',15,15,30,30),(3,'POINT(15 30)',15,15,30,30),(4,NULL,NULL,NULL,NULL,NULL);`, nil)
	p.layers["items"].boundFieldnames = &[4]string{"MINX", "MAXX", "MINY", "MAXY"}
	def, _ := crsconfig.CanonicalFeatureDefinition(3857)
	q := provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, BoundsCRSDefinition: def, Bounds: []geom.Extent{{1669792, 3503549, 1669793, 3503550}}}
	ids, result := queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{2, 3, 4}) || result.NumberMatched == nil || *result.NumberMatched != 3 {
		t.Fatalf("sameSRID definition ignored %v %+v", ids, result)
	}
	q.Offset, q.Limit = 1, 1
	ids, result = queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{3}) || !result.HasMore {
		t.Fatal(ids, result)
	}
	where, _, _, err := featureCandidatePredicate(p.layers["items"], q)
	if err != nil || strings.Contains(where, "MINX") || strings.Contains(where, "rtree_") {
		t.Fatal("unsafe source-space pruning", where, err)
	}
	legacy := q
	legacy.BoundsCRSDefinition = ""
	legacy.Offset, legacy.Limit = 0, 10
	legacyWhere, _, _, err := featureCandidatePredicate(p.layers["items"], legacy)
	if err != nil || !strings.Contains(legacyWhere, "MINX") {
		t.Fatal("negative control did not exercise coarse bounds", legacyWhere, err)
	}
	legacyIDs, _ := queryIDs(t, p, legacy)
	if !reflect.DeepEqual(legacyIDs, []uint64{4}) {
		t.Fatal("numeric Core frame unexpectedly changed", legacyIDs)
	}
	proof, err := p.layers["items"].FeatureCRSDefinition()
	if err != nil || proof.CanonicalAuthority != "EPSG" || proof.CanonicalCode != "4326" {
		t.Fatal("source proof", proof, err)
	}
	proof.Definition = "changed"
	again, _ := p.layers["items"].FeatureCRSDefinition()
	if again.Definition == "changed" {
		t.Fatal("metadata retained")
	}
}

func TestFeatureCRSDefinitionRejectsBeforeIOAndPreservesCore(t *testing.T) {
	p, _ := queryTestProvider(t, `CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT);INSERT INTO items VALUES(1,'POINT(15 30)');`, nil)
	if err := p.db.Close(); err != nil {
		t.Fatal(err)
	}
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsCRSDefinition: "+proj=longlat +datum=NAD83", Bounds: []geom.Extent{{0, 0, 30, 40}}}
	callbacks := 0
	_, err := p.QueryFeatures(context.Background(), "items", q, func(*provider.Feature) error { callbacks++; return nil })
	if !errors.Is(err, provider.ErrUnsupported) || callbacks != 0 {
		t.Fatal("unsupported target reached I/O", err, callbacks)
	}
	layer := &Layer{srid: 4326, geometryFormat: GeometryFormatWKT, spatialMetadata: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	if _, err := layer.FeatureCRSDefinition(); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("unknown proof admitted", err)
	}
	// Unknown optional Part2 source does not alter the established Core path.
	s, err := newSpatialQuery(layer, provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 30, 40}}})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := s.matches(geom.Point{15, 30}, 4326, provider.FeatureQuery{BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 30, 40}}})
	if err != nil || !ok {
		t.Fatal("Core changed", ok, err)
	}
}

func TestFeatureCRSNativeAutomaticOptionalUnsupported(t *testing.T) {
	layer := &Layer{srid: 4326, geometryFormat: GeometryFormatGPKG, spatialMetadata: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	layer.freezeFeatureCRS()
	if _, err := layer.FeatureCRSDefinition(); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("numeric native proof guessed", err)
	}
	if err := layer.FeatureQuerySupported(); err != nil {
		t.Fatal("Core disabled", err)
	}
	layer.crsExplicit = true
	layer.crsConfigured = true
	layer.featureCRSError = nil
	layer.freezeFeatureCRS()
	if _, err := layer.FeatureCRSDefinition(); err != nil {
		t.Fatal("explicit coordinate meaning rejected", err)
	}
}

func TestFeatureCRSRawDefaultOptionalUnsupported(t *testing.T) {
	layer := &Layer{srid: 4326, geometryFormat: GeometryFormatWKT, spatialMetadata: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	layer.freezeFeatureCRS()
	if _, err := layer.FeatureCRSDefinition(); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("default source published", err)
	}
	if err := layer.FeatureQuerySupported(); err != nil {
		t.Fatal("Core disabled", err)
	}
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 30, 40}}}
	s, err := newSpatialQuery(layer, q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := s.matches(geom.Point{15, 30}, 4326, q)
	if err != nil || !matched {
		t.Fatal("Core changed", matched, err)
	}
}

func TestFeatureCRSSemanticIdentityIgnoresNumericMapping(t *testing.T) {
	layer := &Layer{srid: 32633, crsConfigured: true, spatialMetadata: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	layer.freezeFeatureCRS()
	definition, _ := crsconfig.CanonicalFeatureDefinition(32633)
	q := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsCRSDefinition: definition, Bounds: []geom.Extent{{500000, 3318785.352581207, 500000, 3318785.352581207}}}
	s, err := newSpatialQuery(layer, q)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := s.matches(geom.Point{500000, 3318785.352581207}, 32633, q)
	if err != nil || !matched {
		t.Fatal("identity incurred unnecessary roundtrip", matched, err)
	}
}

func TestFeatureCRSFrozenSourceIgnoresRegistryOverride(t *testing.T) {
	previous, _ := basic.EffectiveProj4Definition(3857)
	canonical, _ := crsconfig.CanonicalFeatureDefinition(3857)
	if err := basic.RegisterProj4SRID(3857, canonical); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := basic.RegisterProj4SRID(3857, previous); err != nil {
			t.Error(err)
		}
	}()
	p, _ := queryTestProvider(t, `CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT);INSERT INTO items VALUES(1,'POINT(0 0)'),(2,'POINT(1669792.3618991035 3503549.843504374)');`, map[string]interface{}{"srid": 3857})
	different, _ := crsconfig.CanonicalFeatureDefinition(32633)
	if err := basic.RegisterProj4SRID(3857, different); err != nil {
		t.Fatal(err)
	}
	definition, _ := crsconfig.CanonicalFeatureDefinition(4326)
	q := provider.FeatureQuery{Limit: 10, BoundsSRID: 3857, BoundsCRSDefinition: definition, Bounds: []geom.Extent{{14, 29, 16, 31}}}
	ids, _ := queryIDs(t, p, q)
	if !reflect.DeepEqual(ids, []uint64{2}) {
		t.Fatal("source converter consulted mutable registry", ids)
	}
}

func TestFeatureCRSPolarVertexBboxClamp(t *testing.T) {
	def4326, _ := crsconfig.CanonicalFeatureDefinition(4326)
	def3857, _ := crsconfig.CanonicalFeatureDefinition(3857)
	source, err := crsconfig.NewFeatureProjection(def4326)
	if err != nil {
		t.Fatal(err)
	}
	target, err := crsconfig.NewFeatureProjection(def3857)
	if err != nil {
		t.Fatal(err)
	}
	s := spatialQuery{pinnedSource: source, pinnedTarget: target}
	// Antarctica-like polygon: vertices exactly at the pole have no finite
	// Web Mercator image. The bbox predicate must clamp them to the domain
	// edge instead of failing the whole query.
	antarctica := geom.Polygon{{{0, -90}, {90, -90}, {90, -60}, {0, -60}, {0, -90}}}
	// Canonical full-world EPSG:3857 extent: the rounded ±20037508.34 bound
	// would still exclude a clamped polar vertex, so the test uses the exact
	// canonical edge.
	world := provider.FeatureQuery{BoundsSRID: 3857, Bounds: []geom.Extent{{-20037508.342789244, -20037508.342789244, 20037508.342789244, 20037508.342789244}}}
	matched, err := s.matches(antarctica, 4326, world)
	if err != nil || !matched {
		t.Fatalf("polar bbox predicate: matched=%v err=%v", matched, err)
	}
	// A lone polar point exercises the clamp without any non-polar edge to
	// mask an out-of-extent image: its clamped y must sit inside the world.
	pole := geom.Point{0, -90}
	matched, err = s.matches(pole, 4326, world)
	if err != nil || !matched {
		t.Fatalf("polar point bbox predicate: matched=%v err=%v", matched, err)
	}
	away := provider.FeatureQuery{BoundsSRID: 3857, Bounds: []geom.Extent{{5000000, 5000000, 6000000, 6000000}}}
	matched, err = s.matches(antarctica, 4326, away)
	if err != nil || matched {
		t.Fatalf("negative control: matched=%v err=%v", matched, err)
	}
}
