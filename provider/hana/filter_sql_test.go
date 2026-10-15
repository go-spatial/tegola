package hana

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestFilterCatalogBudgetFailurePreservesCore(t *testing.T) {
	s := featureTestLayer().feature
	for i := 0; i <= provider.MaxQueryableFields; i++ {
		name := fmt.Sprintf("field%d", i)
		s.Catalog.Columns = append(s.Catalog.Columns, featureColumn{Name: name, Type: "INTEGER"})
		s.Projections = append(s.Projections, featureProjection{Output: name, Physical: name})
	}
	err := buildFeatureQueryables(s, featureFilterServerVersion)
	if !errors.Is(err, provider.ErrUnsupported) || s.FilterColumns != nil || s.FilterVersion != "" {
		t.Fatalf("budget failure retained capability: %v", err)
	}
	state := &featureMock{rows: [][]driver.Value{{int64(1), "POINT(0 0)", int64(0), "core"}}}
	p := featureMockProvider(t, state)
	layer := p.layers["features"]
	layer.feature.FilterError = err
	if _, queryableErr := layer.FeatureQueryables(); !errors.Is(queryableErr, provider.ErrUnsupported) {
		t.Fatal(queryableErr)
	}
	called := 0
	if _, coreErr := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { called++; return nil }); coreErr != nil || called != 1 {
		t.Fatalf("optional catalog failure disabled Core: called=%d err=%v", called, coreErr)
	}
}

func TestFilterExactDecimalWireMetadata(t *testing.T) {
	for _, c := range []struct {
		precision, scale int64
		wire             string
		known, want      bool
	}{
		{38, 2, "FIXED16", true, true}, {28, 2, "FIXED12", true, true}, {18, 2, "FIXED8", true, true}, {34, 2, "DECIMAL", true, true},
		{38, 2, "DECIMAL", true, false}, {29, 2, "FIXED12", true, false}, {19, 2, "FIXED8", true, false}, {38, 2, "FIXED16", false, false}, {38, 2, "DOUBLE", true, false},
	} {
		column := featureColumn{Type: "DECIMAL", Length: c.precision, Scale: c.scale}
		if got := featureResultWireCompatible(column, c.wire, c.precision, c.scale, c.known); got != c.want {
			t.Fatalf("wire=%s precision=%d got=%v", c.wire, c.precision, got)
		}
		if featureResultWireCompatible(column, c.wire, c.precision-1, c.scale, c.known) || featureResultWireCompatible(column, c.wire, c.precision, c.scale+1, c.known) {
			t.Fatal("changed result decimal descriptor accepted")
		}
	}
	if featureResultWireCompatible(featureColumn{Type: "DATE"}, "DAYDATE", 0, 0, false) {
		t.Fatal("unproved date wire alias admitted")
	}
}

func TestFilterProtectedExecutorAndVersionDrift(t *testing.T) {
	for _, version := range []string{featureFilterServerVersion, "changed"} {
		t.Run(version, func(t *testing.T) {
			state := &featureMock{filterVersion: version, rows: [][]driver.Value{{int64(2), "POINT(0 0)", int64(0), "selected"}}}
			p := featureMockProvider(t, state)
			layer := p.layers["features"]
			layer.feature.Catalog.Columns[3].Length = 5000
			state.filterColumns = layer.feature.Catalog.Columns
			if err := buildFeatureQueryables(layer.feature, featureFilterServerVersion); err != nil {
				t.Fatal(err)
			}
			filter := filterTestExpression(t, "name", provider.FilterEqual, provider.FilterString, "selected")
			called := 0
			_, err := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Filter: &filter, IDs: []uint64{2}, Limit: 1}, func(*provider.Feature) error { called++; return nil })
			if version != featureFilterServerVersion {
				var data provider.FeatureDataError
				if !errors.As(err, &data) || called != 0 {
					t.Fatalf("version drift called=%d err=%v", called, err)
				}
			} else {
				if err != nil || called != 1 {
					t.Fatalf("called=%d err=%v", called, err)
				}
				found := false
				for _, statement := range state.queries {
					if !strings.Contains(statement, "ORDER BY l.") {
						continue
					}
					found = true
					if !strings.Contains(statement, `STRTOBIN(l."name",'UTF-8') = ?`) || !strings.Contains(statement, `l."id" IN (?)`) {
						t.Fatal(statement)
					}
				}
				if !found {
					t.Fatal("no filtered source SELECT")
				}
				bound := false
				for _, args := range state.args {
					if len(args) == 2 && reflect.DeepEqual(args[0].Value, []byte("selected")) && args[1].Value == int64(2) {
						bound = true
					}
				}
				if !bound {
					t.Fatal("filter/identity parameter order not preserved")
				}
			}
			if !state.rolled || state.closed != 1 {
				t.Fatalf("cleanup rolled=%v closed=%d", state.rolled, state.closed)
			}
		})
	}
}

func filterTestSource(t *testing.T) *featureSource {
	t.Helper()
	s := featureTestLayer().feature
	s.Catalog.Columns = append(s.Catalog.Columns,
		featureColumn{Name: `number".physical`, Type: "DECIMAL", Length: 38, Scale: 2, Nullable: true},
		featureColumn{Name: "boolean", Type: "BOOLEAN", Nullable: true},
		featureColumn{Name: "tiny", Type: "TINYINT", Nullable: true},
		featureColumn{Name: "floating", Type: "DOUBLE"},
		featureColumn{Name: "date", Type: "DATE"},
		featureColumn{Name: "fixed", Type: "NCHAR", Length: 10},
	)
	s.Catalog.Columns[3].Length = 5000
	s.Catalog.Columns[3].Nullable = true
	for _, pair := range [][2]string{{"n", `number".physical`}, {"b", "boolean"}, {"u", "tiny"}, {"floating", "floating"}, {"date", "date"}, {"fixed", "fixed"}} {
		s.Projections = append(s.Projections, featureProjection{Output: pair[0], Physical: pair[1]})
	}
	if err := buildFeatureQueryables(s, featureFilterServerVersion); err != nil {
		t.Fatal(err)
	}
	return s
}

func filterTestExpression(t *testing.T, property string, op provider.FilterCompareOperator, kind provider.FilterScalarType, text string) provider.FilterExpression {
	t.Helper()
	literal, err := provider.NewFilterLiteral(kind, text)
	if err != nil {
		t.Fatal(err)
	}
	expression, err := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterCompare, Property: property, Operator: op, Literal: literal})
	if err != nil {
		t.Fatal(err)
	}
	return expression
}

func TestFilterQueryablesPhysicalPublicationAndOwnership(t *testing.T) {
	s := filterTestSource(t)
	s.Private["tiny"] = true
	s.Public = map[string]bool{"n": true, "b": true, "u": true, "name": true, "floating": true}
	if err := buildFeatureQueryables(s, featureFilterServerVersion); err != nil {
		t.Fatal(err)
	}
	l := Layer{feature: s}
	catalog, err := l.FeatureQueryables()
	if err != nil {
		t.Fatal(err)
	}
	fields := catalog.Fields()
	var names []string
	for _, field := range fields {
		names = append(names, field.Name)
	}
	if !reflect.DeepEqual(names, []string{"b", "n", "name"}) {
		t.Fatal(names)
	}
	fields[0].Name = "changed"
	if _, found := s.Queryables.Lookup("changed"); found {
		t.Fatal("returned catalog aliases retained")
	}
	if err := buildFeatureQueryables(s, "unknown"); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal(err)
	}
	if err := l.FeatureQuerySupported(); err != nil {
		t.Fatal("optional filter profile disabled Core", err)
	}
}

func TestFilterNumericExactQuantizationAndNullFolds(t *testing.T) {
	s := filterTestSource(t)
	for _, c := range []struct {
		property, literal, bound string
		op                       provider.FilterCompareOperator
		fold                     string
	}{
		{"at", "1.1", "2", provider.FilterLess, ""},
		{"at", "1.1", "1", provider.FilterLessEqual, ""},
		{"at", "-1.1", "-2", provider.FilterGreater, ""},
		{"at", "-1.1", "-1", provider.FilterGreaterEqual, ""},
		{"at", "1.1", "", provider.FilterEqual, "ELSE 0 END"},
		{"at", "1.1", "", provider.FilterNotEqual, "ELSE 1 END"},
		{"at", "9223372036854775808", "", provider.FilterLess, "ELSE 1 END"},
		{"at", "-9223372036854775809", "", provider.FilterGreater, "ELSE 1 END"},
		{"at", "1e4096", "", provider.FilterGreaterEqual, "ELSE 0 END"},
		{"u", "-0.1", "", provider.FilterGreater, "ELSE 1 END"},
		{"u", "255.00001", "", provider.FilterLess, "ELSE 1 END"},
		{"n", "-1.001", "-1.01", provider.FilterGreater, ""},
		{"n", "-1.001", "-1.00", provider.FilterGreaterEqual, ""},
		{"n", "1.001", "1.01", provider.FilterLess, ""},
		{"n", "1.001", "1.00", provider.FilterLessEqual, ""},
		{"n", "1.001", "", provider.FilterEqual, "ELSE 0 END"},
		{"n", "1e4096", "", provider.FilterNotEqual, "ELSE 1 END"},
		{"n", "999999999999999999999999999999999999.99", "999999999999999999999999999999999999.99", provider.FilterEqual, ""},
	} {
		t.Run(c.property+"/"+c.literal+"/"+string(rune(c.op+'0')), func(t *testing.T) {
			expression := filterTestExpression(t, c.property, c.op, provider.FilterNumber, c.literal)
			statement, args, err := compileFeatureFilter(s, &expression)
			if err != nil {
				t.Fatal(err)
			}
			if c.fold != "" {
				if len(args) != 0 || !strings.Contains(statement, "IS NULL THEN NULL") || !strings.Contains(statement, c.fold) {
					t.Fatalf("NULL fold %s %#v", statement, args)
				}
				return
			}
			if len(args) != 1 {
				t.Fatal(args)
			}
			if c.property == "n" {
				if args[0] != c.bound || !strings.Contains(statement, `l."number"".physical"`) || !strings.Contains(statement, "CAST(CAST(? AS NVARCHAR(64)) AS DECIMAL(38,2))") {
					t.Fatalf("decimal threshold %s %#v", statement, args)
				}
			} else if got := args[0].(int64); got != map[string]int64{"1": 1, "2": 2, "-1": -1, "-2": -2}[c.bound] {
				t.Fatalf("integer threshold %d want%s", got, c.bound)
			}
		})
	}
}

func TestFilterStringBooleanAllOperatorsAndDetachedArguments(t *testing.T) {
	s := filterTestSource(t)
	text := strings.Repeat("a", 16383) + "\x00"
	for _, op := range []provider.FilterCompareOperator{provider.FilterEqual, provider.FilterNotEqual, provider.FilterLess, provider.FilterLessEqual, provider.FilterGreater, provider.FilterGreaterEqual} {
		for _, property := range []string{"name", "b"} {
			kind, value := provider.FilterString, text
			if property == "b" {
				kind, value = provider.FilterBoolean, "true"
			}
			expression := filterTestExpression(t, property, op, kind, value)
			statement, args, err := compileFeatureFilter(s, &expression)
			if err != nil || len(args) != 1 {
				t.Fatalf("%s %#v %v", statement, args, err)
			}
			if property == "name" {
				if !strings.Contains(statement, "STRTOBIN(") || strings.Contains(statement, "NVARCHAR") || string(args[0].([]byte)) != text {
					t.Fatal(statement, args)
				}
				args[0].([]byte)[0] = 'z'
				_, again, err := compileFeatureFilter(s, &expression)
				if err != nil || string(again[0].([]byte)) != text {
					t.Fatal("arguments alias immutable expression")
				}
			} else if args[0] != int64(1) || !strings.Contains(statement, "IS NULL THEN NULL") {
				t.Fatal(statement, args)
			}
		}
	}
}

func TestFilterInvalidMetadataAndUnpublishedNamesBeforeIO(t *testing.T) {
	for _, c := range []featureColumn{{Type: "DECIMAL", Length: math.MaxInt64, Scale: math.MaxInt64}, {Type: "DECIMAL", Length: 38, Scale: -1}, {Type: "DATE"}} {
		if _, _, _, err := featureNumericDomain(c); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatal(err)
		}
	}
	state := &featureMock{}
	p := featureMockProvider(t, state)
	s := p.layers["features"].feature
	s.Catalog.Columns[3].Length = 5000
	if err := buildFeatureQueryables(s, featureFilterServerVersion); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"id", "geom", "NAME", "name; DROP TABLE T", "unknown"} {
		expression := filterTestExpression(t, name, provider.FilterEqual, provider.FilterString, "secret-value")
		_, err := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1, Filter: &expression}, func(*provider.Feature) error { t.Fatal("callback"); return nil })
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) || len(state.queries) != 0 || strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("unknown/private predicate %v queries%v", err, state.queries)
		}
	}
}

func TestFilterNestedLogicalNullAndParameterOrder(t *testing.T) {
	s := filterTestSource(t)
	left := filterTestExpression(t, "at", provider.FilterEqual, provider.FilterNumber, "0.1")
	right := filterTestExpression(t, "name", provider.FilterGreater, provider.FilterString, "z")
	root := provider.FilterNode{Kind: provider.FilterAnd, Children: []provider.FilterNode{
		{Kind: provider.FilterNot, Children: []provider.FilterNode{left.Root()}},
		{Kind: provider.FilterOr, Children: []provider.FilterNode{right.Root(), {Kind: provider.FilterIsNull, Property: "b"}}},
	}}
	expression, err := provider.NewFilterExpression(root)
	if err != nil {
		t.Fatal(err)
	}
	statement, args, err := compileFeatureFilter(s, &expression)
	if err != nil || !strings.Contains(statement, "NOT (CASE WHEN") || !strings.Contains(statement, " OR ") || !strings.Contains(statement, `l."boolean" IS NULL`) || len(args) != 1 || string(args[0].([]byte)) != "z" {
		t.Fatalf("%s %#v %v", statement, args, err)
	}
}
