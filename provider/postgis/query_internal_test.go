package postgis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
	"github.com/alexeydott/tegola/provider/internal/querytest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type featureTestRows struct {
	data    [][]any
	columns []pgconn.FieldDescription
	index   int
	err     error
	closed  bool
}

func (r *featureTestRows) Close()                                       { r.closed = true }
func (r *featureTestRows) Err() error                                   { return r.err }
func (r *featureTestRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *featureTestRows) FieldDescriptions() []pgconn.FieldDescription { return r.columns }
func (r *featureTestRows) Next() bool {
	if r.index >= len(r.data) {
		return false
	}
	r.index++
	return true
}
func (r *featureTestRows) Values() ([]any, error) { return r.data[r.index-1], nil }
func (r *featureTestRows) RawValues() [][]byte    { return nil }
func (r *featureTestRows) Conn() *pgx.Conn        { return nil }
func (r *featureTestRows) Scan(dest ...any) error { return featureTestScan(dest, r.data[r.index-1]) }
func featureTestScan(dest, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("wrong scan width")
	}
	for i, d := range dest {
		target := reflect.ValueOf(d).Elem()
		value := reflect.ValueOf(values[i])
		if !value.IsValid() {
			return fmt.Errorf("nil scan source")
		}
		if value.Type().AssignableTo(target.Type()) {
			target.Set(value)
		} else if value.Type().ConvertibleTo(target.Type()) {
			target.Set(value.Convert(target.Type()))
		} else {
			return fmt.Errorf("wrong scan type")
		}
	}
	return nil
}
func featureTestProfile() *featureProfile {
	return &featureProfile{schema: "public", table: "items", oid: 42, id: "id", geometry: "geom", format: "wkt", srid: 4326, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}, columns: []featureColumn{{name: "id", number: 1, oid: 20, typeSchema: "pg_catalog"}, {name: "geom", number: 2, oid: 25}, {name: "name", number: 3, oid: 25}, {name: "blob", number: 4, oid: 17}, {name: "start_time", number: 5, oid: 20, typeSchema: "pg_catalog"}, {name: "end_time", number: 6, oid: 20, typeSchema: "pg_catalog"}}}
}
func featureRows(f *featureProfile, columns []string, data ...[]any) *featureTestRows {
	result := &featureTestRows{data: data}
	for _, name := range columns {
		c, _ := f.column(name)
		result.columns = append(result.columns, pgconn.FieldDescription{Name: name, DataTypeOID: c.oid, TableOID: f.oid, TableAttributeNumber: uint16(c.number)})
	}
	return result
}

func TestFeatureTableIdentity(t *testing.T) {
	for _, tc := range []struct{ raw, schema, table string }{{"PUBLIC.Items", "public", "items"}, {`"Mixed"."I""tems"`, "Mixed", `I"tems`}, {"items", "public", "items"}} {
		schema, table, err := featureTableIdentity(tc.raw)
		if err != nil || schema != tc.schema || table != tc.table {
			t.Fatalf("%s => %s.%s %v", tc.raw, schema, table, err)
		}
	}
	for _, raw := range []string{"(SELECT * FROM items) AS x", "public.items;DROP TABLE x", "a.b.c"} {
		if _, _, err := featureTableIdentity(raw); err == nil {
			t.Fatal("expression admitted")
		}
	}
}
func TestFeatureMetadataConfiguration(t *testing.T) {
	for _, conf := range []dict.Dict{{"temporal_field": "time"}, {"temporal_storage": "unix_seconds"}, {"temporal_field": 123}, {"spatial_dimension": "xyz"}, {"spatial_dimension": "bad"}, {"vertical_crs": false}, {"temporal_field": "t", "temporal_start_field": "a", "temporal_end_field": "b", "temporal_storage": "unix_seconds"}} {
		if _, _, _, err := featureConfiguredMetadata(conf); err == nil {
			t.Fatalf("invalid metadata accepted %#v", conf)
		}
	}
	meta, mapping, scale, err := featureConfiguredMetadata(dict.Dict{"spatial_dimension": "xyz", "vertical_crs": provider.CRS84h, "temporal_field": "time", "temporal_storage": "unix_microseconds"})
	if err != nil || meta.Dimension != provider.DimensionXYZ || mapping.InstantField != "time" || scale != 1000000 {
		t.Fatal(meta, mapping, scale, err)
	}
}
func TestFeatureCandidateSQL(t *testing.T) {
	f := featureTestProfile()
	f.format = ""
	f.postgisSchema = "spatial"
	cursor := int64(8)
	statement, args := f.chunkStatement(provider.FeatureQuery{IDs: []uint64{9, math.MaxUint64}, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326}, []string{"id", "geom", "name"}, &cursor)
	for _, required := range []string{`FROM ONLY "public"."items"`, `"spatial".ST_AsBinary(l."geom")`, `"spatial".ST_IsEmpty`, `l."id" IN ($1)`, `l."id">$7`, `ORDER BY l."id" LIMIT 256`} {
		if !strings.Contains(statement, required) {
			t.Fatalf("missing %s: %s", required, statement)
		}
	}
	if len(args) != 7 || args[0] != int64(9) || args[6] != int64(8) {
		t.Fatal(args)
	}
	q := provider.FeatureQuery{Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 3857}
	statement, args = f.chunkStatement(q, []string{"id", "geom"}, nil)
	if strings.Contains(statement, "ST_MakeEnvelope") || len(args) != 0 {
		t.Fatal("nonlinear inverse pruning")
	}
}
func TestFeatureExactPredicatesBeforePagingAndOwnership(t *testing.T) {
	f := featureTestProfile()
	columns := []string{"id", "geom", "blob"}
	blob := []byte{7, 8}
	rows := featureRows(f, columns, []any{int64(1), "POINT(20 20)", blob}, []any{int64(2), "POINT(0 0)", blob}, []any{int64(3), "POINT(1 1)", blob}, []any{int64(4), nil, blob})
	q := provider.FeatureQuery{Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326, Offset: 1, Limit: 1}
	var cursor *int64
	var matches uint64
	var result provider.FeatureQueryResult
	var ids []uint64
	_, stop, err := consumeFeatureChunk(context.Background(), rows, f, q, columns, []string{"blob"}, &cursor, &matches, &result, func(feature *provider.Feature) error {
		ids = append(ids, feature.ID)
		feature.Tags["blob"].([]byte)[0] = 99
		return nil
	})
	if err != nil || !stop || !result.HasMore || result.NumberReturned != 1 || !reflect.DeepEqual(ids, []uint64{3}) || blob[0] != 7 {
		t.Fatal(ids, result, blob, err)
	}
}
func TestFeatureRowErrorsCancellationAndLineage(t *testing.T) {
	f := featureTestProfile()
	columns := []string{"id", "geom"}
	for _, data := range [][]any{{int64(-1), "POINT(0 0)"}, {int64(1), "broken"}, {int64(1), "POINT Z(0 0 1)"}} {
		rows := featureRows(f, columns, data)
		var cursor *int64
		var matches uint64
		var result provider.FeatureQueryResult
		_, _, err := consumeFeatureChunk(context.Background(), rows, f, provider.FeatureQuery{Limit: 1}, columns, nil, &cursor, &matches, &result, func(*provider.Feature) error { return nil })
		var source provider.FeatureDataError
		if !errors.As(err, &source) {
			t.Fatalf("corruption unclassified %v", err)
		}
	}
	rows := featureRows(f, columns, []any{int64(1), "POINT(0 0)"})
	rows.columns[0].TableOID = 99
	var cursor *int64
	var matches uint64
	var result provider.FeatureQueryResult
	if _, _, err := consumeFeatureChunk(context.Background(), rows, f, provider.FeatureQuery{Limit: 1}, columns, nil, &cursor, &matches, &result, func(*provider.Feature) error { t.Fatal("callback after lineage drift"); return nil }); err == nil {
		t.Fatal("lineage drift accepted")
	}
	rows = featureRows(f, columns, []any{int64(1), "POINT(0 0)"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result = provider.FeatureQueryResult{}
	if _, _, err := consumeFeatureChunk(ctx, rows, f, provider.FeatureQuery{Limit: 1}, columns, nil, &cursor, &matches, &result, func(*provider.Feature) error { cancel(); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("final callback cancellation lost: %v", err)
	}
}
func TestFeatureExactTemporalBoundaries(t *testing.T) {
	f := featureTestProfile()
	f.temporal = provider.TemporalMapping{InstantField: "time"}
	f.temporalScale = 1000000000
	instant := time.Unix(-1, 999999999).UTC()
	q := &provider.TemporalConstraint{Start: &instant, End: &instant, StartSubNanosecond: "1", EndSubNanosecond: "1"}
	matched, err := f.matchesFeatureTemporal(map[string]any{"time": int64(-1)}, q)
	if err != nil || matched {
		t.Fatal(matched, err)
	}
	matched, err = f.matchesFeatureTemporal(map[string]any{"time": nil}, q)
	if err != nil || !matched {
		t.Fatal("absent temporal geometry lost", err)
	}
	leap := time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC)
	f.temporalScale = 1
	q = &provider.TemporalConstraint{Start: &leap, End: &leap, StartLeapSecond: true, EndLeapSecond: true}
	for _, value := range []int64{leap.Unix(), leap.Unix() + 1} {
		matched, err = f.matchesFeatureTemporal(map[string]any{"time": value}, q)
		if err != nil || matched {
			t.Fatal("leap instant matched POSIX coordinate", err)
		}
	}
}
func TestFeatureUnsupportedPropertiesRemainExplicit(t *testing.T) {
	f := featureTestProfile()
	f.columns = append(f.columns, featureColumn{name: "created", oid: 1184})
	if _, err := f.featureFields(nil); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("unsupported property silently omitted: %v", err)
	}
	f.private = map[string]bool{"created": true}
	if _, err := f.featureFields(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := detachFeatureProperty(math.NaN()); err == nil {
		t.Fatal("nonfinite property admitted")
	}
}
func (r *featureTestRows) TypeMap() *pgtype.Map { return pgtype.NewMap() }

type featureTestRow struct {
	values []any
	err    error
}

func (r featureTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return featureTestScan(dest, r.values)
}

type featureTestCatalog struct{}

func (featureTestCatalog) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected catalog query")
}
func (featureTestCatalog) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "pg_index") {
		return featureTestRow{values: []any{true}}
	}
	if strings.Contains(query, "pg_collation") {
		return featureTestRow{values: []any{"pg_catalog", "C"}}
	}
	return featureTestRow{err: fmt.Errorf("unexpected catalog row")}
}

func TestFeatureSQLAliasesDomainAndBinding(t *testing.T) {
	f := featureTestProfile()
	f.id = "public_id"
	f.geometry = "shape"
	f.private = map[string]bool{"blob": true}
	f.temporal = provider.TemporalMapping{InstantField: "observed"}
	f.temporalScale = 1
	plan, err := featuresql.Parse(`SELECT f.id AS public_id,f.geom AS shape,f.name AS title,f.blob AS hidden,f.start_time AS observed FROM public.items AS f WHERE (f.id>=1e1 AND f.id<>12) OR f.name='can''t'`, featuresql.PostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	for i := range f.columns {
		if f.columns[i].name == "name" {
			f.columns[i].collation = 100
		}
	}
	if err := resolvePostGISFeatureSelection(context.Background(), featureTestCatalog{}, f, plan); err != nil {
		t.Fatal(err)
	}
	fields, err := f.featureFields(nil)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(fields, []string{"hidden"}) || strings.Contains(strings.Join(fields, ","), "hidden") {
		t.Fatal("physical private lineage leaked", fields)
	}
	statement, args := f.chunkStatement(provider.FeatureQuery{IDs: []uint64{15}}, f.queryColumns(fields), nil)
	if strings.Contains(statement, "can't") || !strings.Contains(statement, `l."geom" AS "shape"`) || !strings.Contains(statement, `l."id" IN ($4)`) || len(args) != 4 || args[0] != int64(10) || args[2] != "can't" || args[3] != int64(15) {
		t.Fatal(statement, args)
	}
	if _, err := f.featureFields([]string{"hidden"}); err == nil {
		t.Fatal("private alias selectable")
	}
}
func TestFeatureLiteralIntegerPrecisionAndBounds(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int64
	}{{"9007199254740993", 9007199254740993}, {"-9223372036854775808", math.MinInt64}, {"1.23000e2", 123}, {"+1e3", 1000}, {"0e99999999999999999999", 0}, {"1.000", 1}} {
		value, err := featureLiteralInteger(tc.raw)
		if err != nil || value != tc.want {
			t.Fatal(tc.raw, value, err)
		}
	}
	for _, raw := range []string{"9223372036854775808", "1.1", "1e99999999999999999999", "1e-99999999999999999999", "1e-1"} {
		if _, err := featureLiteralInteger(raw); err == nil {
			t.Fatal("invalid literal admitted", raw)
		}
	}
}

func TestFeatureFloatingPredicateAdmission(t *testing.T) {
	for _, tc := range []struct {
		oid     uint32
		literal string
	}{{700, "16777217"}, {701, "9007199254740993"}, {701, "0.1"}, {700, "1"}} {
		f := featureTestProfile()
		f.columns = append(f.columns, featureColumn{name: "value", number: 7, oid: tc.oid})
		plan, err := featuresql.Parse(`SELECT id,geom,value FROM public.items WHERE value=`+tc.literal, featuresql.PostgreSQL)
		if err != nil {
			t.Fatal(err)
		}
		err = resolvePostGISFeatureSelection(context.Background(), featureTestCatalog{}, f, plan)
		if !errors.Is(err, provider.ErrUnsupported) || f.where != "" || len(f.whereArgs) != 0 {
			t.Fatal("rounded floating comparison was admitted", tc, err)
		}
		f.err = err
		p := &Provider{layers: map[string]Layer{"items": {feature: f}}}
		_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error {
			t.Fatal("unsupported profile reached source callback")
			return nil
		})
		if !errors.Is(err, provider.ErrUnsupported) || !errors.Is((Layer{feature: f}).FeatureQuerySupported(), provider.ErrUnsupported) {
			t.Fatal("unsupported predicate reached transaction I/O", err)
		}
	}
}

func TestFeatureReservedPhysicalCaseAndAliases(t *testing.T) {
	for _, name := range []string{"MIN_ZOOM", "Max_Zoom", "min_zoom", "MAX_ZOOM"} {
		f := featureTestProfile()
		f.columns = append(f.columns, featureColumn{name: name, number: 7, oid: 23})
		f.projections = []featureProjection{{output: "renamed", column: f.columns[6]}}
		fields, err := f.featureFields(nil)
		if err != nil || len(fields) != 0 {
			t.Fatal("reserved physical alias leaked", name, fields, err)
		}
		if _, err := f.featureFields([]string{"renamed"}); err == nil {
			t.Fatal("reserved physical alias was requestable", name)
		}
		f.projections = nil
		if _, err := f.featureFields([]string{name}); err == nil {
			t.Fatal("reserved physical property was requestable", name)
		}
	}
	f := featureTestProfile()
	f.private = map[string]bool{"MixedBounds": true}
	if !f.privatePhysicalField("MixedBounds") || f.privatePhysicalField("mixedbounds") {
		t.Fatal("configured bounds catalog identity was folded")
	}
}

func TestFeatureRequestedNonplanarityAndSourceIntegrity(t *testing.T) {
	f := featureTestProfile()
	f.spatial = provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}
	var err error
	f.height, err = crsconfig.NewHeightProjection(f.srid)
	if err != nil {
		t.Fatal(err)
	}
	q := provider.FeatureQuery{Limit: 1, Bounds3D: []provider.Extent3D{{-1, -1, -1, 200000, 200000, 10}}, BoundsSRID: 3857, BoundsVerticalCRS: provider.CRS84h}
	for _, tc := range []struct {
		body        string
		unsupported bool
	}{
		{"POLYGON Z ((0 0 0,1 0 1,1 1 2,0 0.5 0.5,0 0 0))", true},
		{"POLYGON Z ((0 0 0,1 0 1,1 1 2,0 0.5 9,0 0 0))", false},
	} {
		rows := featureRows(f, []string{"id", "geom"}, []any{int64(1), tc.body})
		var cursor *int64
		var matches uint64
		var result provider.FeatureQueryResult
		callbacks := 0
		_, _, err := consumeFeatureChunk(context.Background(), rows, f, q, []string{"id", "geom"}, nil, &cursor, &matches, &result, func(*provider.Feature) error { callbacks++; return nil })
		var source provider.FeatureDataError
		if callbacks != 0 || errors.As(err, &source) == tc.unsupported || (!tc.unsupported && err == nil) || (tc.unsupported && !errors.Is(err, provider.ErrUnsupported)) {
			t.Fatal("source integrity and requested spatial profile conflated", tc, callbacks, err)
		}
	}
}

func TestFeatureTemporalReadsDoNotExpandRequestedProperties(t *testing.T) {
	f := featureTestProfile()
	f.temporal = provider.TemporalMapping{StartField: "start_time", EndField: "end_time"}
	f.temporalScale = 1000000000
	fields, err := f.featureFields([]string{"name"})
	if err != nil {
		t.Fatal(err)
	}
	columns := f.queryColumns(fields)
	if !reflect.DeepEqual(columns, []string{"id", "geom", "start_time", "end_time", "name"}) {
		t.Fatal("private temporal reads are missing", columns)
	}
	rows := featureRows(f, columns, []any{int64(1), "POINT(0 0)", int64(0), int64(100), "only-public"})
	var cursor *int64
	var matches uint64
	var result provider.FeatureQueryResult
	callbacks := 0
	_, _, err = consumeFeatureChunk(context.Background(), rows, f, provider.FeatureQuery{Limit: 1, Fields: []string{"name"}}, columns, fields, &cursor, &matches, &result, func(feature *provider.Feature) error {
		callbacks++
		if !reflect.DeepEqual(feature.Tags, map[string]interface{}{"name": "only-public"}) {
			t.Fatal("required temporal reads leaked into requested properties", feature.Tags)
		}
		return nil
	})
	if err != nil || callbacks != 1 {
		t.Fatal(callbacks, err)
	}
}

func TestFeatureMandatoryOnlySelectionHasEmptyPublicProperties(t *testing.T) {
	f := featureTestProfile()
	f.publicFields = []string{"id", "geom"}
	fields, err := f.featureFields(nil)
	if err != nil || len(fields) != 0 {
		t.Fatal("mandatory-only selection exposed public properties", fields, err)
	}
	if _, err := f.featureFields([]string{"name"}); err == nil {
		t.Fatal("empty public schema exposed an unselected property")
	}
}

func TestFeatureValidatedEmptySourceNormalization(t *testing.T) {
	for _, format := range []string{"wkb", ""} {
		f := featureTestProfile()
		f.format = format
		f.spatial = provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}
		for _, source := range []geom.Geometry{geom.LineStringZ{}, geom.MultiLineStringZ{geom.LineStringZ{}}, geom.Collection{geom.Collection{geom.LineStringZ{}}}} {
			wire, err := querytest.EncodeFixtureWKB(source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.decodeFeatureGeometry(wire)
			if err != nil || got != nil {
				t.Fatal("wholly empty validated source did not become absence", format, source, got, err)
			}
		}
		for _, source := range []geom.Geometry{
			geom.Collection{geom.LineStringZ{}, geom.PointZ{15, 30, 7}},
			geom.Collection{geom.LineStringZ{}, geom.PointZ{15, math.NaN(), 7}},
			geom.Collection{geom.LineStringZ{}, geom.Point{15, 30}},
		} {
			wire, err := querytest.EncodeFixtureWKB(source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.decodeFeatureGeometry(wire)
			valid := reflect.DeepEqual(source, geom.Collection{geom.LineStringZ{}, geom.PointZ{15, 30, 7}})
			if valid && (err != nil || got == nil) || !valid && err == nil {
				t.Fatal("empty child hid source evidence", format, source, got, err)
			}
		}
	}
	f := featureTestProfile()
	f.spatial = provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}
	if got, err := f.decodeFeatureGeometry("GEOMETRYCOLLECTION Z (LINESTRING Z EMPTY)"); err != nil || got != nil {
		t.Fatal("empty WKT source did not become absence", got, err)
	}
	f.format = "wkb"
	f.columns[1].oid = 17
	wire, err := querytest.EncodeFixtureWKB(geom.Collection{geom.LineStringZ{}})
	if err != nil {
		t.Fatal(err)
	}
	rows := featureRows(f, []string{"id", "geom"}, []any{int64(1), wire})
	var cursor *int64
	var matches uint64
	var result provider.FeatureQueryResult
	callbacks := 0
	_, _, err = consumeFeatureChunk(context.Background(), rows, f, provider.FeatureQuery{Limit: 1, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h, Bounds3D: []provider.Extent3D{{100, 100, 100, 101, 101, 101}}}, []string{"id", "geom"}, nil, &cursor, &matches, &result, func(feature *provider.Feature) error {
		callbacks++
		if feature.Geometry != nil {
			t.Fatal("empty source callback retained geometry", feature.Geometry)
		}
		return nil
	})
	if err != nil || callbacks != 1 || result.NumberReturned != 1 {
		t.Fatal("empty source failed absent-space/count policy", callbacks, result, err)
	}
}

type featureTestSnapshot struct {
	profile *featureProfile
	data    [][]any
	steps   []string
	drift   bool
	closed  int
}

func (s *featureTestSnapshot) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	s.steps = append(s.steps, "lock")
	if !strings.HasPrefix(query, "LOCK TABLE ONLY") {
		return pgconn.CommandTag{}, fmt.Errorf("unexpected exec")
	}
	return pgconn.CommandTag{}, nil
}
func (s *featureTestSnapshot) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	s.steps = append(s.steps, "catalog")
	switch {
	case strings.Contains(query, "pg_class c"):
		oid := s.profile.oid
		if s.drift {
			oid++
		}
		return featureTestRow{values: []any{oid, "r", "p", false, false}}
	case strings.Contains(query, "pg_inherits"):
		return featureTestRow{values: []any{false, false}}
	case strings.Contains(query, "pg_index"):
		return featureTestRow{values: []any{true}}
	}
	return featureTestRow{err: fmt.Errorf("unexpected catalog row")}
}
func (s *featureTestSnapshot) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "pg_attribute") {
		s.steps = append(s.steps, "columns")
		var data [][]any
		for _, c := range s.profile.columns {
			data = append(data, []any{c.name, c.number, c.oid, c.typmod, c.generated, c.identity, c.collation, c.notNull, c.typeName, c.typeSchema, c.extension})
		}
		return &featureTestRows{data: data}, nil
	}
	s.steps = append(s.steps, "source")
	cursor := int64(-1)
	if len(args) > 0 {
		cursor = args[len(args)-1].(int64)
	}
	var data [][]any
	for _, row := range s.data {
		if row[0].(int64) > cursor {
			data = append(data, row)
			if len(data) == 256 {
				break
			}
		}
	}
	return featureRows(s.profile, []string{"id", "geom"}, data...), nil
}
func TestFeatureProtectedSnapshotChunksAndDrift(t *testing.T) {
	f := featureTestProfile()
	snapshot := &featureTestSnapshot{profile: f}
	for i := range 265 {
		snapshot.data = append(snapshot.data, []any{int64(i), "POINT(0 0)"})
	}
	callbacks := 0
	result, err := executeFeatureSnapshot(context.Background(), snapshot, f, provider.FeatureQuery{Limit: 300}, nil, func(*provider.Feature) error { callbacks++; return nil })
	if err != nil || callbacks != 265 || result.NumberReturned != 265 || result.HasMore {
		t.Fatal(callbacks, result, err)
	}
	if len(snapshot.steps) < 6 || snapshot.steps[0] != "lock" || snapshot.steps[1] != "catalog" {
		t.Fatal("callbacks lacked source protection", snapshot.steps)
	}
	snapshot = &featureTestSnapshot{profile: f, drift: true}
	callbacks = 0
	_, err = executeFeatureSnapshot(context.Background(), snapshot, f, provider.FeatureQuery{Limit: 1}, nil, func(*provider.Feature) error { callbacks++; return nil })
	var source provider.FeatureDataError
	if !errors.As(err, &source) || callbacks != 0 {
		t.Fatal("replacement table was read", callbacks, err)
	}
}

func TestFeatureIdentifierUnicodeFolding(t *testing.T) {
	if got := featureFoldIdentifier("ÄABC"); got != "Äabc" {
		t.Fatal("UTF8 PostgreSQL identifier folding changed a multibyte codepoint", got)
	}
}

func TestFeatureIdentifierLimitAndPublicSubset(t *testing.T) {
	f := featureTestProfile()
	r := featureSQLCatalog{profile: f}
	plan, err := featuresql.Parse(`SELECT id AS `+strings.Repeat("x", 64)+` FROM items`, featuresql.PostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.OutputKey(plan.Projections()[0].Output()); err == nil {
		t.Fatal("identifier truncation was admitted")
	}
	f.columns = append(f.columns, featureColumn{name: "opaque", oid: 1700})
	f.publicFields = []string{"name", "start_time"}
	fields, err := f.featureFields(nil)
	if err != nil || !reflect.DeepEqual(fields, []string{"name", "start_time"}) {
		t.Fatal(fields, err)
	}
	if _, err := f.featureFields([]string{"opaque"}); err == nil {
		t.Fatal("unpublished property was exposed")
	}
	f.publicFields = nil
	if _, err := f.featureFields(nil); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("unsupported all-property profile was silently narrowed", err)
	}
	f.projections = []featureProjection{
		{output: "public_id", column: f.columns[0]},
		{output: "renamed_zoom", column: featureColumn{name: "min_zoom", oid: 23}},
		{output: "observed", column: f.columns[4]},
	}
	f.id = "public_id"
	f.private = map[string]bool{"min_zoom": true, "max_zoom": true}
	fields, err = f.featureFields(nil)
	if err != nil || !reflect.DeepEqual(fields, []string{"observed"}) {
		t.Fatal("physical private lineage or public temporal projection changed", fields, err)
	}
}
