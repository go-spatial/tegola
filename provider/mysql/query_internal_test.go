package mysql

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func testFeatureProfile(t *testing.T, unsigned bool) *featureProfile {
	t.Helper()
	typeName := "bigint"
	if unsigned {
		typeName += " unsigned"
	}
	f := &featureProfile{schema: featureSchema{database: "gis", table: "items", engine: "InnoDB", tableType: "BASE TABLE",
		columns: []featureColumn{{name: "id", dataType: "bigint", columnType: typeName}, {name: "geom", dataType: "text"},
			{name: "name", dataType: "varchar"}, {name: "start", dataType: "bigint"}, {name: "end", dataType: "bigint"}},
		indexes: []featureIndexColumn{{name: "PRIMARY", column: "id", position: 1}}},
		id: "id", geometry: "geom", format: GeometryFormatWKT, srid: 4326, filter: "1",
		spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	if err := f.resolveOrdinary(Layer{tagFieldnames: []string{"name"}}); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFeatureStrictNativeWireAndProperties(t *testing.T) {
	f := testFeatureProfile(t, false)
	f.native, f.format = true, GeometryFormatMySQL
	f.nativeSRIDLabel = "__private_srid"
	_, _, _, err := f.decodeFeature([]string{"id", "geom", "__private_srid"}, []any{int64(1), []byte{0xff}, int64(4326)}, nil)
	if err == nil {
		t.Fatal("malformed exported native WKB accepted")
	}
	for _, test := range []struct{ dataType, value string }{{"decimal", "null"}, {"decimal", "true"}, {"double", "NaN"}, {"double", "+Inf"}} {
		if _, err := featureProperty(featureColumn{dataType: test.dataType}, []byte(test.value)); err == nil {
			t.Errorf("invalid property %s/%s accepted", test.dataType, test.value)
		}
	}
	for _, name := range []string{"min_zoom", "MAX_ZOOM", "BOUND.Min"} {
		if !featurePrivateColumn(Layer{bboxFields: [4]string{"bound.min"}}, name) {
			t.Errorf("private physical column %s exposed", name)
		}
	}
}

func TestFeatureStrictRawEmptyXYZNormalization(t *testing.T) {
	f := testFeatureProfile(t, false)
	f.format = GeometryFormatWKB
	f.spatial = provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}
	for _, source := range []geom.Geometry{geom.LineStringZ{}, geom.Collection{geom.LineStringZ{}, geom.Collection{geom.PolygonZ{}}}} {
		wire, err := querytest.EncodeFixtureWKB(source)
		if err != nil {
			t.Fatal(err)
		}
		feature, _, representable, err := f.decodeFeature([]string{"id", "geom"}, []any{int64(1), wire}, nil)
		if err != nil || !representable || feature.Geometry != nil {
			t.Fatalf("whollyempty source must decode to canonical nil: %#v %v", feature.Geometry, err)
		}
	}
	for _, source := range []geom.Geometry{
		geom.Collection{geom.LineStringZ{}, geom.PointZ{1, 2, 3}},
		geom.Collection{geom.LineStringZ{}, geom.PointZ{math.NaN(), 2, 3}},
	} {
		wire, err := querytest.EncodeFixtureWKB(source)
		if err != nil {
			t.Fatal(err)
		}
		feature, _, _, err := f.decodeFeature([]string{"id", "geom"}, []any{int64(1), wire}, nil)
		invalid := math.IsNaN(source.(geom.Collection)[1].(geom.PointZ)[0])
		if invalid && err == nil {
			t.Fatal("empty first child masked invalid later child")
		}
		if !invalid && (err != nil || feature.Geometry == nil) {
			t.Fatalf("empty child erased nonempty sibling: %#v %v", feature.Geometry, err)
		}
	}
}

func TestFeatureNativeMariaDBVersionEvidenceGate(t *testing.T) {
	for _, version := range []string{"10.7.4", "10.7.4-MariaDB-log", "10.11.8-MariaDB", "11.4.2-MariaDB"} {
		if !featureNativeMariaDBVersion(version) {
			t.Errorf("evidenced native version rejected: %s", version)
		}
	}
	for _, version := range []string{"10.7", "10.7.40-MariaDB", "10.7.5-MariaDB", "10.7.3", "10.8.4", "11.3.4", "10.7.4.1", "10.7.4bad-MariaDB", "10.7.-4", "10.7.+4", "+10.7.4", "10.+7.4", "10..4", ".7.4", "10.7.", "10.7.999999999999999999999999", "unknown"} {
		if featureNativeMariaDBVersion(version) {
			t.Errorf("unevidenced native version admitted: %s", version)
		}
	}
}

func TestFeatureIdentityProofAndUnsigned(t *testing.T) {
	f := testFeatureProfile(t, true)
	for _, value := range []any{uint64(math.MaxUint64), []byte("18446744073709551615")} {
		id, err := f.decodeID(value)
		if err != nil || id != math.MaxUint64 {
			t.Fatalf("unsigned identity: %d %v", id, err)
		}
	}
	for _, value := range []any{float64(1), int64(-1), []byte("18446744073709551616")} {
		if _, err := f.decodeID(value); err == nil {
			t.Fatalf("accepted invalid identity %T", value)
		}
	}
	where, args, impossible := f.candidatePredicate([]uint64{math.MaxUint64, math.MaxUint64, 0})
	if impossible || !strings.Contains(where, "IN (?,?)") || !reflect.DeepEqual(args, []any{uint64(math.MaxUint64), uint64(0)}) {
		t.Fatalf("unsigned bindings: %s %#v", where, args)
	}
	f = testFeatureProfile(t, false)
	_, _, impossible = f.candidatePredicate([]uint64{math.MaxUint64})
	if !impossible {
		t.Fatal("signed overflow did not produce empty match")
	}
	for _, change := range []func(*featureProfile){
		func(f *featureProfile) {
			f.schema.indexes = append(f.schema.indexes, featureIndexColumn{name: "PRIMARY", column: "name", position: 2})
		},
		func(f *featureProfile) { f.schema.indexes[0].nonUnique = 1 },
		func(f *featureProfile) { f.schema.indexes[0].prefix.Valid = true },
		func(f *featureProfile) { f.schema.columns[0].extra = "VIRTUAL GENERATED" },
		func(f *featureProfile) { f.schema.columns[0].dataType = "double" },
	} {
		candidate := testFeatureProfile(t, false)
		change(candidate)
		if err := candidate.resolveOrdinary(Layer{}); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("unproven identity accepted: %v", err)
		}
	}
	f = testFeatureProfile(t, false)
	f.schema.columns[0].extra = "auto_increment"
	if err := f.resolveOrdinary(Layer{}); err != nil {
		t.Fatalf("stored auto increment rejected: %v", err)
	}
}

func TestFeatureTemporalExactBounds(t *testing.T) {
	for _, scale := range []int64{1, 1000, 1000000, 1000000000} {
		t.Run(strconv.FormatInt(scale, 10), func(t *testing.T) {
			instant := time.Unix(-1, 0)
			if got := exactFeatureEpochBound(instant, scale, true, "1", false); got.String() != strconv.FormatInt(-scale+1, 10) {
				t.Fatal(got)
			}
			if got := exactFeatureEpochBound(instant, scale, false, "1", false); got.String() != strconv.FormatInt(-scale, 10) {
				t.Fatal(got)
			}
			leap := time.Date(2016, 12, 31, 23, 59, 59, 123, time.UTC)
			if got := exactFeatureEpochBound(leap, scale, true, "01", true); got.String() != strconv.FormatInt(1483228800*scale, 10) {
				t.Fatal(got)
			}
			if got := exactFeatureEpochBound(leap, scale, false, "01", true); got.String() != strconv.FormatInt(1483228800*scale-1, 10) {
				t.Fatal(got)
			}
		})
	}
	f := testFeatureProfile(t, false)
	if err := f.registerTemporal(dict.Dict{"temporal_start_field": "start", "temporal_end_field": "end", "temporal_storage": "unix_nanoseconds"}); err != nil {
		t.Fatal(err)
	}
	instant := time.Unix(0, 1)
	query := &provider.TemporalConstraint{Start: &instant, End: &instant, StartSubNanosecond: "1", EndSubNanosecond: "1"}
	match, err := f.temporalMatches(map[string]any{"start": int64(1), "end": int64(1)}, query)
	if err != nil || match {
		t.Fatalf("subnanosecond rounded: %v %v", match, err)
	}
	match, err = f.temporalMatches(map[string]any{"start": int64(1), "end": int64(2)}, query)
	if err != nil || !match {
		t.Fatalf("exact interval: %v %v", match, err)
	}
	match, err = f.temporalMatches(map[string]any{}, query)
	if err != nil || !match {
		t.Fatalf("absence: %v %v", match, err)
	}
	if _, err := f.temporalMatches(map[string]any{"start": int64(2), "end": int64(1)}, nil); err == nil {
		t.Fatal("reversed interval escaped no-query validation")
	}
	if _, err := f.temporalMatches(map[string]any{"start": "1"}, nil); err == nil {
		t.Fatal("text interval coerced")
	}
}

func TestFeatureStrictDecodeAndDimensionalSpatial(t *testing.T) {
	f := testFeatureProfile(t, false)
	columns := []string{"id", "geom", "name"}
	values := []any{int64(1), "POINT(1 2)", []byte("road")}
	feature, match, representable, err := f.decodeFeature(columns, values, nil)
	if err != nil || !match || !representable || feature.Tags["name"] != "road" {
		t.Fatalf("decode: %#v %v", feature, err)
	}
	feature.Tags["name"] = "changed"
	if string(values[2].([]byte)) != "road" {
		t.Fatal("input retained")
	}
	for _, bad := range []any{"POINT Z(1 2 3)", "POINT(1 NaN)", "POINT(1 2) garbage"} {
		values[1] = bad
		if _, _, _, err := f.decodeFeature(columns, values, nil); err == nil {
			t.Fatalf("accepted invalid raw geometry %v", bad)
		}
	}
	values[1] = nil
	feature, _, representable, err = f.decodeFeature(columns, values, nil)
	if err != nil || !representable || feature.Geometry != nil {
		t.Fatalf("null geometry %v", err)
	}
	f.spatial = provider.SpatialMetadata{Dimension: provider.DimensionMixedXYXYZ, VerticalCRS: provider.CRS84h}
	query := provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h,
		Bounds3D: []provider.Extent3D{{0, 0, 100, 2, 3, 101}}}
	spatial, err := newFeatureSpatialQuery(f, query)
	if err != nil {
		t.Fatal(err)
	}
	for _, geometry := range []geom.Geometry{geom.Point{1, 2}, nil} {
		match, err := spatial.matches(geometry, 4326, query)
		if err != nil || !match {
			t.Fatalf("XY/absence policy: %v %v", match, err)
		}
	}
	match, err = spatial.matches(geom.PointZ{1, 2, 3}, 4326, query)
	if err != nil || match {
		t.Fatalf("height dropped: %v %v", match, err)
	}
}

func TestFeatureCapabilityAndRequestBeforeIO(t *testing.T) {
	f := testFeatureProfile(t, false)
	layer := Layer{name: "items", srid: 3857, feature: f}
	p := &Provider{layers: map[string]Layer{"items": layer}}
	if layer.FeatureSourceSRID() != 4326 || layer.SRID() != 3857 {
		t.Fatal("tile CRS changed")
	}
	for _, query := range []provider.FeatureQuery{{Limit: 0}, {Limit: 1, Fields: []string{"private"}},
		{Limit: 1, BoundsSRID: 4326, Bounds3D: []provider.Extent3D{{0, 0, 0, 1, 1, 1}}, BoundsVerticalCRS: "unknown"}} {
		if _, err := p.QueryFeatures(context.Background(), "items", query, func(*provider.Feature) error { t.Fatal("unexpected callback"); return nil }); err == nil {
			t.Fatal("invalid request reached IO")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	wrapped := featureDataError(featureInvalid("geometry", "bad source"))
	var data provider.FeatureDataError
	if !errors.As(wrapped, &data) {
		t.Fatal("source error not classified")
	}
	if !errors.Is(featureDataError(context.Canceled), context.Canceled) {
		t.Fatal("context chain lost")
	}
}
