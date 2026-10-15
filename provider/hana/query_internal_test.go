package hana

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

func TestFeatureAtomicQuoting(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{`"secret"`, `"""secret"""`}, {"a.b", `"a.b"`}, {`a"b`, `"a""b"`},
	} {
		if got := quoteIdent(tc.name); got != tc.want {
			t.Fatalf("%q -> %q want %q", tc.name, got, tc.want)
		}
	}
	c := featureCatalog{Schema: `"schema"`, Table: "a.b"}
	if got := c.qualified(); got != `"""schema"""."a.b"` {
		t.Fatal(got)
	}
}

func featureTestLayer() Layer {
	catalog := featureCatalog{Schema: "S", Table: "T", OID: 1, UniqueKeys: [][]string{{"id"}}, Columns: []featureColumn{
		{Name: "id", ID: 1, Type: "BIGINT"}, {Name: "geom", ID: 2, Type: "VARCHAR"}, {Name: "at", ID: 3, Type: "BIGINT"}, {Name: "name", ID: 4, Type: "NVARCHAR"},
	}}
	return Layer{idField: "id", geomField: "geom", geometryFormat: "wkt", featureSRID: 4326, feature: &featureSource{
		Catalog: catalog, ID: "id", Geometry: "geom", SRID: 4326, Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY},
		Temporal: provider.TemporalMapping{InstantField: "at"}, TemporalScale: 1, Private: map[string]bool{"id": true, "geom": true},
		Projections: []featureProjection{{"id", "id"}, {"geom", "geom"}, {"at", "at"}, {"name", "name"}},
	}}
}

func TestFeatureSourceDimensionDecodeAndPrivateLineage(t *testing.T) {
	l := featureTestLayer()
	l.feature.Projections = append(l.feature.Projections, featureProjection{"secretAlias", "geom"})
	row := map[string]any{"id": int64(3), "geom": "POINT(1 2)", "at": int64(0), "name": "detached"}
	f, ok, err := decodeFeature(l, row, map[string]bool{"name": true, "secretAlias": true})
	if err != nil || !ok || f.ID != 3 || f.SRID != 4326 {
		t.Fatalf("%+v %v %v", f, ok, err)
	}
	if _, leaked := f.Tags["secretAlias"]; leaked || len(f.Tags) != 1 {
		t.Fatal(f.Tags)
	}
	row["geom"] = "POINT Z(1 2 3)"
	if _, _, err := decodeFeature(l, row, nil); err == nil {
		t.Fatal("XYZ accepted as XY")
	}
	row["id"] = int64(-1)
	if _, _, err := decodeFeature(l, row, nil); err == nil {
		t.Fatal("negative identity accepted")
	}
	row["id"] = nil
	if _, ok, err := decodeFeature(l, row, nil); err != nil || ok {
		t.Fatalf("null ID must skip: %v %v", ok, err)
	}
}

func TestFeatureExactTemporalBounds(t *testing.T) {
	before := time.Unix(-1, 999999999)
	for _, scale := range []int64{1, 1000, 1000000, 1000000000} {
		floor := exactEpochBound(before, scale, false, "1", false)
		ceil := exactEpochBound(before, scale, true, "1", false)
		if floor.Int64() != -1 || ceil.Sign() != 0 {
			t.Fatalf("scale %d floor=%v ceil=%v", scale, floor, ceil)
		}
		leapFloor := exactEpochBound(time.Unix(100, 0), scale, false, "", true)
		leapCeil := exactEpochBound(time.Unix(100, 0), scale, true, "9", true)
		if leapFloor.Int64() != 101*scale-1 || leapCeil.Int64() != 101*scale {
			t.Fatal(leapFloor, leapCeil)
		}
	}
	l := featureTestLayer()
	start := time.Unix(0, 1)
	var args []any
	sql := temporalPredicate(l.feature, &provider.TemporalConstraint{Start: &start}, &args)
	if !strings.Contains(sql, `l."at">=?`) || len(args) != 1 || args[0] != int64(1) {
		t.Fatalf("%s %+v", sql, args)
	}
}

func TestFeatureConfigFailures(t *testing.T) {
	for _, conf := range []dict.Dict{
		{"vertical_crs": "  "}, {"spatial_dimension": "xyz"}, {"temporal_field": "at"}, {"temporal_storage": "unix_seconds"},
	} {
		l := Layer{featureSRID: 4326}
		_, err := configureFeatureSource(&l, conf)
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("%v: %v", conf, err)
		}
	}
}

func TestFeatureSpatialFullValidation(t *testing.T) {
	l := featureTestLayer()
	q := provider.FeatureQuery{Bounds: []geom.Extent{{0, 0, 3, 3}}, BoundsSRID: 4326}
	s, err := newSpatialQuery(&l, q)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.matches(geom.Collection{geom.Point{1, 1}, geom.Point{math.NaN(), 2}}, 4326, q)
	if err == nil {
		t.Fatal("matching sibling hid corruption")
	}
}

func TestFeatureSQLFrozenLineageAndBoundValues(t *testing.T) {
	l := featureTestLayer()
	plan, err := featuresql.Parse(`SELECT "id" AS "id", "geom" AS "geom", "at" AS "at", "name" AS "label" FROM "S"."T" AS t WHERE t."at">=-1 AND t."name"='quote''value'`, featuresql.HANA)
	if err != nil {
		t.Fatal(err)
	}
	s := *l.feature
	s.Projections = nil
	if err := resolveFeatureSQL(&l, &s, plan); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.BasePredicate, "quote") || len(s.BaseArgs) != 2 || s.BaseArgs[0] != int64(-1) || s.BaseArgs[1] != "quote'value" {
		t.Fatalf("%s %#v", s.BasePredicate, s.BaseArgs)
	}
	if s.Projections[3].Output != "label" || s.Projections[3].Physical != "name" {
		t.Fatal(s.Projections)
	}
	if err := resolveFeatureSource(&l, &s); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{`SELECT id, geom FROM "S"."T"`, `SELECT "id" AS "same","geom" AS "same" FROM "S"."T"`} {
		plan, err := featuresql.Parse(sql, featuresql.HANA)
		if err != nil {
			t.Fatal(err)
		}
		candidate := *l.feature
		candidate.Projections = nil
		if err := resolveFeatureSQL(&l, &candidate, plan); err == nil {
			t.Fatalf("unproven lineage admitted %s", sql)
		}
	}
}

func TestFeatureSQLMalformedConfigPrecedesCapability(t *testing.T) {
	for _, value := range []any{12, " ", "SELECT x; SELECT y", "SELECT !BBOX! FROM x", "SELECT xyz nonsense"} {
		l := Layer{featureSRID: 4326}
		p := Provider{}
		err := p.registerFeatureSource(&l, dict.Dict{"feature_sql": value}, ProviderType)
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("invalid explicit SQL lost typed error: %v", err)
		}
	}
}

func TestFeatureSQLTypedLiteralLimits(t *testing.T) {
	catalog := featureSQLCatalog{featureTestLayer().feature.Catalog}
	for _, sql := range []string{`SELECT "id","geom" FROM "S"."T" WHERE "id"=9223372036854775808`, `SELECT "id","geom" FROM "S"."T" WHERE "id"=1.1`} {
		plan, err := featuresql.Parse(sql, featuresql.HANA)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := featuresql.Resolve(plan, catalog, featuresql.ResolveOptions{IdentityOutput: "id", GeometryOutput: "geom"})
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, err = featuresql.CompileWhere(resolved, featuresql.RenderOptions{QuoteIdentifier: quoteIdent, Placeholder: func(int) string { return "?" }, BindLiteral: bindFeatureSQLLiteral, FirstParameter: 1})
		if !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("unsafe narrowing accepted: %v", err)
		}
	}
}

func TestFeatureSQLDecimalCharacterTransport(t *testing.T) {
	l := featureTestLayer()
	l.feature.Catalog.Columns = append(l.feature.Catalog.Columns, featureColumn{Name: "amount", ID: 5, Type: "DECIMAL", Length: 38, Scale: 9})
	for _, literal := range []string{"12345678901234567890123456789.123456789", "-12345678901234567890123456789.123456789"} {
		plan, err := featuresql.Parse(`SELECT "id","geom","amount" FROM "S"."T" WHERE "amount"=`+literal, featuresql.HANA)
		if err != nil {
			t.Fatal(err)
		}
		s := *l.feature
		s.Projections = nil
		s.Temporal = provider.TemporalMapping{}
		if err := resolveFeatureSQL(&l, &s, plan); err != nil {
			t.Fatal(err)
		}
		if len(s.BaseArgs) != 1 || s.BaseArgs[0] != literal || !strings.Contains(s.BasePredicate, "CAST(CAST(? AS NVARCHAR(64)) AS DECIMAL(38,9))") {
			t.Fatalf("%s %#v", s.BasePredicate, s.BaseArgs)
		}
	}
}

func TestFeatureSQLNumericAllocationBounds(t *testing.T) {
	for _, text := range []string{"1e1000000000", "1e-1000000000", "1e999999999999999999999999999999999999", "1e-999999999999999999999999999999999999"} {
		if r, ok := featureBoundedNumber(text); ok || r != nil {
			t.Fatalf("huge exponent admitted %s", text)
		}
		if allocations := testing.AllocsPerRun(10, func() { _, _ = featureBoundedNumber(text) }); allocations != 0 {
			t.Fatalf("huge exponent allocated arithmetic objects: %g", allocations)
		}
	}
	for _, text := range []string{"0e1000000000", "-0e-1000000000", "1" + strings.Repeat("0", 60000) + "e-60000", "0." + strings.Repeat("0", 60000) + "1e60001"} {
		r, ok := featureBoundedNumber(text)
		if !ok {
			t.Fatal("safe zero/coefficient cancellation rejected")
		}
		if strings.HasPrefix(text, "0e") || strings.HasPrefix(text, "-0e") {
			if r.Sign() != 0 {
				t.Fatal(r)
			}
		} else if r.Num().Int64() != 1 || !r.IsInt() {
			t.Fatal(r)
		}
	}
	plan, err := featuresql.Parse(`SELECT "id","geom" FROM "S"."T" WHERE "id"=1`, featuresql.HANA)
	if err != nil {
		t.Fatal(err)
	}
	predicate, _ := plan.Predicate()
	literal := predicate.Literals()[0]
	for _, column := range []featuresql.ColumnMetadata{
		{Kind: featuresql.IntegerColumn, Bits: 0}, {Kind: featuresql.IntegerColumn, Bits: -1}, {Kind: featuresql.IntegerColumn, Bits: 1000000000},
		{Kind: featuresql.IntegerColumn, Bits: 64, Unsigned: true},
		{Kind: featuresql.DecimalColumn, Precision: 38, Scale: 1000000000}, {Kind: featuresql.DecimalColumn, Precision: 1, Scale: 2},
		{Kind: featuresql.DecimalColumn, Precision: 1000000000, Scale: 0},
	} {
		if _, err := bindFeatureSQLLiteral(column, "=", literal); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("invalid metadata admitted %+v: %v", column, err)
		}
	}
	for _, literalText := range []string{"1e1000000000", "1e-1000000000"} {
		l := featureTestLayer()
		plan, err := featuresql.Parse(`SELECT "id","geom" FROM "S"."T" WHERE "id"=`+literalText, featuresql.HANA)
		if err != nil {
			t.Fatal(err)
		}
		s := *l.feature
		s.Projections = nil
		s.Temporal = provider.TemporalMapping{}
		if err := resolveFeatureSQL(&l, &s, plan); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("registration path admitted huge exponent: %v", err)
		}
	}
}
