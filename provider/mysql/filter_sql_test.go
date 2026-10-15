package mysql

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestFeatureFilterNumericLattice(t *testing.T) {
	for _, test := range []struct {
		literal  string
		operator provider.FilterCompareOperator
		unsigned bool
		want     string
		argument string
	}{
		{"-1.2", provider.FilterLess, false, "<=", "-2"},
		{"-1.2", provider.FilterGreaterEqual, false, ">=", "-1"},
		{"1.2", provider.FilterLessEqual, false, "<=", "1"},
		{"1.2", provider.FilterGreater, false, ">=", "2"},
		{"18446744073709551615", provider.FilterEqual, true, "=", "18446744073709551615"},
		{"18446744073709551616", provider.FilterEqual, true, "ELSE 0", ""},
		{"1.1", provider.FilterNotEqual, false, "ELSE 1", ""},
		{"-9223372036854775809", provider.FilterLess, false, "ELSE 0", ""},
		{"9223372036854775808", provider.FilterLess, false, "ELSE 1", ""},
	} {
		literal, err := provider.NewFilterLiteral(provider.FilterNumber, test.literal)
		if err != nil {
			t.Fatal(err)
		}
		args := []any{}
		c := featureFilterColumn{kind: provider.QueryableInteger, bits: 64, unsigned: test.unsigned}
		sql, err := c.compileNumber("l.`n`", test.operator, literal, &args)
		if err != nil || !strings.Contains(sql, test.want) {
			t.Fatalf("%+v: %q %v", test, sql, err)
		}
		if test.argument == "" {
			if len(args) != 0 || !strings.Contains(sql, "IS NULL THEN NULL") {
				t.Fatalf("fold lost UNKNOWN: %s %#v", sql, args)
			}
		} else if !reflect.DeepEqual(args, []any{test.argument}) {
			t.Fatalf("exact arg: %#v", args)
		}
		if test.unsigned && len(args) != 0 && !strings.Contains(sql, "CAST(? AS DECIMAL(20,0))") {
			t.Fatalf("unsigned parameter can narrow before casting: %s", sql)
		}
	}
}

func TestFeatureFilterDecimalLattice(t *testing.T) {
	for _, test := range []struct {
		literal, argument string
		operator          provider.FilterCompareOperator
	}{
		{"-1.234", "-1.24", provider.FilterLess},
		{"-1.234", "-1.23", provider.FilterGreaterEqual},
		{"999999999999999999.99", "999999999999999999.99", provider.FilterEqual},
	} {
		literal, _ := provider.NewFilterLiteral(provider.FilterNumber, test.literal)
		args := []any{}
		c := featureFilterColumn{kind: provider.QueryableNumber, precision: 20, scale: 2}
		sql, err := c.compileNumber("l.`d`", test.operator, literal, &args)
		if err != nil || !strings.Contains(sql, "CAST(? AS DECIMAL(20,2))") || !reflect.DeepEqual(args, []any{test.argument}) {
			t.Fatalf("decimal: %s %#v %v", sql, args, err)
		}
	}
}

func TestFeatureFilterCatalogAndDetachedCompilation(t *testing.T) {
	f := &featureProfile{
		properties: []string{"n", "s", "b", "float", "char", "generated"},
		projection: []featureProjection{{"source_n", "n"}, {"source_s", "s"}, {"source_b", "b"}, {"f", "float"}, {"ch", "char"}, {"g", "generated"}},
		schema: featureSchema{columns: []featureColumn{
			{name: "source_n", dataType: "bigint", columnType: "bigint unsigned", nullable: "YES"},
			{name: "source_s", dataType: "varchar", characterSet: "utf8mb4", nullable: "YES"},
			{name: "source_b", dataType: "bit", columnType: "bit(1)", nullable: "YES"},
			{name: "f", dataType: "double"}, {name: "ch", dataType: "char", characterSet: "utf8mb4"},
			{name: "g", dataType: "int", extra: "VIRTUAL GENERATED"},
		}},
		filter: "l.`selection_flag`=1", filterArgs: []any{},
	}
	f.initializeFilterCatalog()
	if got := f.queryableCatalog.Fields(); len(got) != 3 {
		t.Fatalf("catalog: %#v", got)
	}
	literal, _ := provider.NewFilterLiteral(provider.FilterString, "x\x00🙂  ")
	expression, _ := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterCompare, Property: "s", Operator: provider.FilterEqual, Literal: literal})
	compiled, err := f.prepareFeatureFilter(&expression)
	if err != nil {
		t.Fatal(err)
	}
	if compiled == f || f.filter != "l.`selection_flag`=1" || len(f.filterArgs) != 0 ||
		!strings.Contains(compiled.filter, "CAST(l.`source_s` AS BINARY) = UNHEX(?)") ||
		!reflect.DeepEqual(compiled.filterArgs, []any{"7800f09f99822020"}) {
		t.Fatalf("detached byte predicate: %#v %s", compiled.filterArgs, compiled.filter)
	}
	private, _ := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterIsNull, Property: "source_s"})
	_, err = f.prepareFeatureFilter(&private)
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) {
		t.Fatalf("private name: %v", err)
	}
}

func TestFeatureFilterOptionalCatalogBudgetPreservesCore(t *testing.T) {
	f := &featureProfile{}
	for i := 0; i <= provider.MaxQueryableFields; i++ {
		name := strings.Repeat("x", i/100+1) + string(rune(0x1000+i))
		f.properties = append(f.properties, name)
		f.projection = append(f.projection, featureProjection{name, name})
		f.schema.columns = append(f.schema.columns, featureColumn{name: name, dataType: "int", columnType: "int"})
	}
	f.initializeFilterCatalog()
	l := Layer{feature: f}
	if err := l.FeatureQuerySupported(); err != nil {
		t.Fatalf("Core lost: %v", err)
	}
	if _, err := l.FeatureQueryables(); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("budget not closed: %v", err)
	}
}

func TestFeatureRawBooleanProjectionStrict(t *testing.T) {
	c := featureColumn{name: "b", dataType: "bit", columnType: "bit(1)"}
	for _, test := range []struct {
		raw  any
		want any
	}{{nil, nil}, {int64(0), false}, {uint64(1), true}} {
		value, err := featureProperty(c, test.raw)
		if err != nil || value != test.want {
			t.Fatalf("boolean: %#v %v", value, err)
		}
	}
	for _, value := range []any{int64(-1), int64(2), []byte{1}} {
		if _, err := featureProperty(c, value); err == nil {
			t.Fatalf("invalid bool accepted: %#v", value)
		}
	}
}
