package postgis

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func filterTestProfile(t *testing.T) *featureProfile {
	t.Helper()
	f := featureTestProfile()
	f.columns = append(f.columns,
		featureColumn{name: "n", number: 7, oid: 20, typeSchema: "pg_catalog"},
		featureColumn{name: "s", number: 8, oid: 25, typeSchema: "pg_catalog"},
		featureColumn{name: "b", number: 9, oid: 16, typeSchema: "pg_catalog"},
		featureColumn{name: "padded", number: 10, oid: 1042, typeSchema: "pg_catalog"},
		featureColumn{name: "float", number: 11, oid: 701, typeSchema: "pg_catalog"},
		featureColumn{name: "Max_Zoom", number: 12, oid: 20, typeSchema: "pg_catalog"})
	f.publicFields = []string{"n", "s", "b", "padded", "float"}
	if err := f.freezeQueryables(); err != nil {
		t.Fatal(err)
	}
	return f
}
func filterTestExpression(t *testing.T, node provider.FilterNode) *provider.FilterExpression {
	t.Helper()
	e, err := provider.NewFilterExpression(node)
	if err != nil {
		t.Fatal(err)
	}
	return &e
}
func filterTestCompare(t *testing.T, name, text string, kind provider.FilterScalarType, op provider.FilterCompareOperator) provider.FilterNode {
	t.Helper()
	literal, err := provider.NewFilterLiteral(kind, text)
	if err != nil {
		t.Fatal(err)
	}
	return provider.FilterNode{Kind: provider.FilterCompare, Property: name, Literal: literal, Operator: op}
}

func TestFeatureFilterQueryablesSnapshot(t *testing.T) {
	f := filterTestProfile(t)
	layer := Layer{feature: f}
	q, err := layer.FeatureQueryables()
	if err != nil {
		t.Fatal(err)
	}
	fields := q.Fields()
	if !reflect.DeepEqual(fields, []provider.FeatureQueryable{{Name: "b", Type: provider.QueryableBoolean, Nullable: true}, {Name: "n", Type: provider.QueryableInteger, Nullable: true}, {Name: "s", Type: provider.QueryableString, Nullable: true}}) {
		t.Fatalf("catalog=%v", fields)
	}
	fields[0].Name = "private"
	again, err := layer.FeatureQueryables()
	if err != nil {
		t.Fatal(err)
	}
	if again.Fields()[0].Name != "b" {
		t.Fatal("catalog escaped immutable snapshot")
	}
	f.projections = []featureProjection{{output: "alias", column: f.columns[6]}, {output: "hidden", column: f.columns[11]}}
	f.publicFields = nil
	if err := f.freezeQueryables(); err != nil {
		t.Fatal(err)
	}
	if got := f.queryables.Fields(); len(got) != 1 || got[0].Name != "alias" {
		t.Fatalf("alias/private lineage=%v", got)
	}
}

func TestFeatureFilterExactCompilation(t *testing.T) {
	f := filterTestProfile(t)
	f.where = `l."n" >= $1`
	f.whereArgs = []any{int64(-10)}
	for _, literal := range []string{"0.1", "-1.5", "9223372036854775808", "-9223372036854775809", "1e4096", "-1e4096", "1e-4096", "-1e-4096"} {
		for op := provider.FilterEqual; op <= provider.FilterGreaterEqual; op++ {
			node := filterTestCompare(t, "n", literal, provider.FilterNumber, op)
			expression := filterTestExpression(t, provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{node}})
			prepared, err := f.prepareFeatureFilter(expression)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(prepared.where, "NOT (") || !strings.Contains(prepared.where, `l."n"::pg_catalog.numeric OPERATOR(pg_catalog.`) || !strings.Contains(prepared.where, "$2::pg_catalog.text::pg_catalog.numeric") {
				t.Fatalf("lost nullable exact expression: %s", prepared.where)
			}
			if !reflect.DeepEqual(prepared.whereArgs, []any{int64(-10), literal}) {
				t.Fatal("literal narrowed")
			}
		}
	}
	if f.where != `l."n" >= $1` || !reflect.DeepEqual(f.whereArgs, []any{int64(-10)}) {
		t.Fatal("registered selection mutated")
	}
	for _, value := range []string{"", "a ", "😀é\x00", "'; DROP TABLE source;--", strings.Repeat("z", 16384)} {
		prepared, err := f.prepareFeatureFilter(filterTestExpression(t, filterTestCompare(t, "s", value, provider.FilterString, provider.FilterGreaterEqual)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(prepared.where, value) && value != "" && len(value) > 10 {
			t.Fatal("literal entered SQL")
		}
		if !strings.Contains(prepared.where, `pg_catalog.convert_to(l."s",'UTF8')`) || !strings.Contains(prepared.where, "$2::pg_catalog.bytea") {
			t.Fatal("bytewise semantics absent")
		}
		if !reflect.DeepEqual(prepared.whereArgs[1], []byte(value)) {
			t.Fatal("UTF8 literal changed")
		}
	}
	for op := provider.FilterEqual; op <= provider.FilterGreaterEqual; op++ {
		prepared, err := f.prepareFeatureFilter(filterTestExpression(t, filterTestCompare(t, "b", "true", provider.FilterBoolean, op)))
		if err != nil {
			t.Fatal(err)
		}
		if prepared.whereArgs[1] != true || !strings.Contains(prepared.where, "::pg_catalog.bool") {
			t.Fatal("boolean changed")
		}
	}
	for _, kind := range []provider.FilterKind{provider.FilterIsNull, provider.FilterIsNotNull} {
		p, err := f.prepareFeatureFilter(filterTestExpression(t, provider.FilterNode{Kind: kind, Property: "n"}))
		if err != nil {
			t.Fatal(err)
		}
		if len(p.whereArgs) != 1 || !strings.Contains(p.where, "IS ") {
			t.Fatal("null predicate wrong")
		}
	}
}

func TestFeatureFilterRejectsBeforeSourceIO(t *testing.T) {
	f := filterTestProfile(t)
	p := Provider{layers: map[string]Layer{"items": {feature: f}}}
	for _, name := range []string{"id", "geom", "Max_Zoom", "padded", "float", "n;DROP"} {
		e := filterTestExpression(t, filterTestCompare(t, name, "1", provider.FilterNumber, provider.FilterEqual))
		_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1, Filter: e}, func(*provider.Feature) error { return nil })
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("property %q did not fail before nil pool: %v", name, err)
		}
	}
}

func TestFeatureFilterOptionalCatalogPreservesCore(t *testing.T) {
	for _, budget := range []string{"count", "aggregate names"} {
		t.Run(budget, func(t *testing.T) {
			f := featureTestProfile()
			f.columns = nil
			count := provider.MaxQueryableFields + 1
			nameSize := 8
			if budget == "aggregate names" {
				count = 1024
				nameSize = 70
			}
			for i := 0; i < count; i++ {
				name := fmt.Sprintf("field_%04d_", i) + strings.Repeat("x", nameSize)
				f.columns = append(f.columns, featureColumn{name: name, oid: 20, typeSchema: "pg_catalog"})
			}
			f.freezeOptionalQueryables()
			layer := Layer{feature: f}
			if err := layer.FeatureQuerySupported(); err != nil {
				t.Fatalf("Core disabled: %v", err)
			}
			if _, err := layer.FeatureQueryables(); !errors.Is(err, provider.ErrUnsupported) {
				t.Fatalf("optional catalog=%v", err)
			}
			if got, err := f.prepareFeatureFilter(nil); err != nil || got != f {
				t.Fatal("nil filter altered Core")
			}
			if _, err := f.featureFields(nil); err != nil {
				t.Fatal("Core source fields changed")
			}
			if f.err != nil {
				t.Fatal("source error set by optional capability")
			}
		})
	}
	f := featureTestProfile()
	f.err = featureUnsupported("underlying source unavailable")
	f.freezeOptionalQueryables()
	if !errors.Is((Layer{feature: f}).FeatureQuerySupported(), provider.ErrUnsupported) {
		t.Fatal("invalid source became admitted")
	}
}
