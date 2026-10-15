package provider

import (
	"errors"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func testFilterLiteral(t *testing.T, kind FilterScalarType, text string) FilterLiteral {
	t.Helper()
	literal, err := NewFilterLiteral(kind, text)
	if err != nil {
		t.Fatal(err)
	}
	return literal
}
func testFilterExpression(t *testing.T, node FilterNode) FilterExpression {
	t.Helper()
	expression, err := NewFilterExpression(node)
	if err != nil {
		t.Fatal(err)
	}
	return expression
}

func TestFilterLiteralExactValues(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"18446744073709551615", "18446744073709551615"},
		{"-9223372036854775808", "-9223372036854775808"},
		{"1.25e-2", "1/80"},
		{"+.5", "1/2"},
		{"-0", "0"},
	} {
		literal := testFilterLiteral(t, FilterNumber, tc.text)
		got, ok := literal.Number()
		if !ok || got.RatString() != tc.want || literal.Text() != tc.text {
			t.Fatalf("%s: %v", tc.text, got)
		}
		got.SetInt64(42)
		again, ok := literal.Number()
		if !ok || again.RatString() != tc.want {
			t.Fatal("number getter retains mutable value")
		}
	}
	literal := testFilterLiteral(t, FilterString, "é e\u0301  ")
	if literal.Text() != "é e\u0301  " {
		t.Fatal("string normalization changed value")
	}
	if _, ok := literal.Number(); ok {
		t.Fatal("string implicitly converted")
	}
	for _, kind := range []FilterScalarType{FilterBoolean, FilterDate, FilterTimestamp} {
		text := map[FilterScalarType]string{
			FilterBoolean: "false", FilterDate: "2024-02-29",
			FilterTimestamp: "2016-12-31T23:59:60.123456789012Z",
		}[kind]
		literal := testFilterLiteral(t, kind, text)
		if literal.Type() != kind || literal.Text() != text {
			t.Fatal("literal changed")
		}
		if kind == FilterTimestamp {
			instant, ok := literal.Instant()
			if !ok || !instant.LeapSecond || instant.SubNanosecond != "012" ||
				instant.Time.Nanosecond() != 123456789 || instant.Time.Second() != 59 {
				t.Fatalf("exact instant: %#v", instant)
			}
		}
		if kind == FilterDate {
			date, ok := literal.Date()
			if !ok || date.Format("2006-01-02") != text {
				t.Fatal(date)
			}
		}
	}
}

func TestFilterLiteralRejectsInvalidAndBounded(t *testing.T) {
	for _, tc := range []struct {
		kind FilterScalarType
		text string
	}{
		{FilterScalarInvalid, ""},
		{FilterNumber, "NaN"}, {FilterNumber, "Infinity"}, {FilterNumber, "0x10"}, {FilterNumber, "1/2"},
		{FilterNumber, "1e4097"}, {FilterNumber, "1e-4097"}, {FilterNumber, "1e999999999999999999999"},
		{FilterNumber, strings.Repeat("1", MaxFilterNumericDigits+1)},
		{FilterBoolean, "TRUE"}, {FilterBoolean, "1"},
		{FilterDate, "2023-02-29"}, {FilterDate, "2024-2-29"},
		{FilterTimestamp, "2024-02-29T12:00:00+00:00"},
		{FilterTimestamp, "2024-02-29t12:00:00z"},
		{FilterTimestamp, "2024-02-29T12:00:00.Z"},
		{FilterTimestamp, "2024-02-29T12:00:60Z"},
		{FilterTimestamp, "2017-12-31T23:59:60Z"},
		{FilterTimestamp, "2024-01-01T00:00:00." + strings.Repeat("0", MaxFilterTimestampFractionDigits+1) + "Z"},
		{FilterString, string([]byte{0xff})}, {FilterString, strings.Repeat("a", MaxFilterLiteralBytes+1)},
	} {
		if _, err := NewFilterLiteral(tc.kind, tc.text); err == nil {
			t.Fatalf("accepted type=%d literal length=%d", tc.kind, len(tc.text))
		}
	}
	for _, text := range []string{"1e4096", "1e-4096", strings.Repeat("1", MaxFilterNumericDigits)} {
		testFilterLiteral(t, FilterNumber, text)
	}
	testFilterLiteral(t, FilterString, strings.Repeat("a", MaxFilterLiteralBytes))
	testFilterLiteral(t, FilterTimestamp, "2024-01-01T00:00:00."+strings.Repeat("0", MaxFilterTimestampFractionDigits)+"Z")
}

func TestFilterNodeShapesAndUnsupported(t *testing.T) {
	literal := testFilterLiteral(t, FilterNumber, "1")
	compare := FilterNode{Kind: FilterCompare, Property: "value", Operator: FilterEqual, Literal: literal}
	for _, node := range []FilterNode{
		{Kind: FilterBooleanConstant}, {Kind: FilterBooleanConstant, Boolean: true},
		compare, {Kind: FilterIsNull, Property: "value"}, {Kind: FilterIsNotNull, Property: "value"},
		{Kind: FilterNot, Children: []FilterNode{compare}},
		{Kind: FilterAnd, Children: []FilterNode{compare, compare}},
		{Kind: FilterOr, Children: []FilterNode{compare, compare}},
	} {
		expression := testFilterExpression(t, node)
		if err := expression.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, node := range []FilterNode{
		{}, {Kind: FilterKind(255)},
		{Kind: FilterCompare, Property: "value", Operator: FilterEqual},
		{Kind: FilterCompare, Property: " ", Operator: FilterEqual, Literal: literal},
		{Kind: FilterCompare, Property: "value", Operator: FilterCompareOperator(255), Literal: literal},
		{Kind: FilterIsNull, Property: "value", Literal: literal},
		{Kind: FilterBooleanConstant, Property: "value"},
		{Kind: FilterAnd, Children: []FilterNode{compare}},
		{Kind: FilterNot, Children: []FilterNode{compare, compare}},
	} {
		if _, err := NewFilterExpression(node); err == nil {
			t.Fatalf("accepted shape %#v", node)
		}
	}
	for _, kind := range []FilterKind{FilterLike, FilterIn, FilterBetween, FilterSpatial, FilterTemporal} {
		_, err := NewFilterExpression(FilterNode{Kind: kind})
		var invalid InvalidFeatureQueryError
		if !errors.Is(err, ErrUnsupported) || !errors.As(err, &invalid) {
			t.Fatalf("unsupported kind %d: %v", kind, err)
		}
	}
	if err := (FilterExpression{}).Validate(); err == nil {
		t.Fatal("zero expression valid")
	}
	if err := (FeatureQuery{Limit: 1, Filter: &FilterExpression{}}).Validate(); err == nil {
		t.Fatal("query ignored invalid filter")
	}
}

func TestFilterExpressionResourceBounds(t *testing.T) {
	leaf := FilterNode{Kind: FilterBooleanConstant}
	chain := leaf
	for i := 1; i < MaxFilterDepth; i++ {
		chain = FilterNode{Kind: FilterNot, Children: []FilterNode{chain}}
	}
	testFilterExpression(t, chain)
	if _, err := NewFilterExpression(FilterNode{Kind: FilterNot, Children: []FilterNode{chain}}); err == nil {
		t.Fatal("depth limit ignored")
	}
	cycle := FilterNode{Kind: FilterNot, Children: make([]FilterNode, 1)}
	cycle.Children[0] = cycle
	if _, err := NewFilterExpression(cycle); err == nil {
		t.Fatal("cycle accepted")
	}
	children := make([]FilterNode, MaxFilterNodes-1)
	for i := range children {
		children[i] = leaf
	}
	testFilterExpression(t, FilterNode{Kind: FilterAnd, Children: children})
	children = append(children, leaf)
	if _, err := NewFilterExpression(FilterNode{Kind: FilterAnd, Children: children}); err == nil {
		t.Fatal("node limit ignored")
	}
	literal := testFilterLiteral(t, FilterString, strings.Repeat("a", MaxFilterLiteralBytes))
	large := FilterNode{Kind: FilterCompare, Property: "n", Operator: FilterEqual, Literal: literal}
	largeTree := FilterNode{Kind: FilterAnd, Children: []FilterNode{large, large, large, large}}
	if _, err := NewFilterExpression(largeTree); err == nil {
		t.Fatal("aggregate bytes ignored")
	}
	property := strings.Repeat("x", MaxFilterPropertyBytes)
	testFilterExpression(t, FilterNode{Kind: FilterIsNull, Property: property})
	byteBoundary := make([]FilterNode, MaxFilterBytes/MaxFilterPropertyBytes)
	for i := range byteBoundary {
		byteBoundary[i] = FilterNode{Kind: FilterIsNull, Property: property}
	}
	testFilterExpression(t, FilterNode{Kind: FilterAnd, Children: byteBoundary})
	byteBoundary = append(byteBoundary, FilterNode{Kind: FilterIsNull, Property: "n"})
	if _, err := NewFilterExpression(FilterNode{Kind: FilterAnd, Children: byteBoundary}); err == nil {
		t.Fatal("exact aggregate byte boundary ignored")
	}
	if _, err := NewFilterExpression(FilterNode{Kind: FilterIsNull, Property: property + "x"}); err == nil {
		t.Fatal("property bytes ignored")
	}
}

func TestFilterCatalogResolutionAndOwnership(t *testing.T) {
	fields := []FeatureQueryable{
		{Name: "n", Type: QueryableInteger, Nullable: true},
		{Name: "s", Type: QueryableString},
		{Name: "bool", Type: QueryableBoolean},
		{Name: "date", Type: QueryableDate},
		{Name: "time", Type: QueryableTimestamp},
		{Name: "decimal", Type: QueryableNumber},
		{Name: "odd.name", Type: QueryableString},
	}
	catalog, err := NewFeatureQueryables(fields)
	if err != nil {
		t.Fatal(err)
	}
	fields[0].Name = "changed"
	snapshot := catalog.Fields()
	snapshot[0].Name = "changed"
	if _, ok := catalog.Lookup("n"); !ok {
		t.Fatal("catalog retained caller/getter fields")
	}
	if _, ok := catalog.Lookup("N"); ok {
		t.Fatal("case folding inferred")
	}
	if _, ok := catalog.Lookup("name"); ok {
		t.Fatal("dot qualification inferred")
	}
	for _, tc := range []struct {
		property string
		kind     FilterScalarType
		text     string
	}{
		{"n", FilterNumber, "1.5"}, {"decimal", FilterNumber, "18446744073709551616"},
		{"s", FilterString, "a  "}, {"bool", FilterBoolean, "false"},
		{"date", FilterDate, "2024-02-29"}, {"time", FilterTimestamp, "2016-12-31T23:59:60.12345678901Z"},
		{"odd.name", FilterString, "safe"},
	} {
		for operator := FilterEqual; operator <= FilterGreaterEqual; operator++ {
			expression := testFilterExpression(t, FilterNode{
				Kind: FilterCompare, Property: tc.property, Operator: operator,
				Literal: testFilterLiteral(t, tc.kind, tc.text),
			})
			if _, err := ResolveFeatureFilter(expression, catalog); err != nil {
				t.Fatalf("%s operator=%d: %v", tc.property, operator, err)
			}
		}
	}
	for _, property := range []string{"n", "s"} {
		for _, kind := range []FilterKind{FilterIsNull, FilterIsNotNull} {
			expression := testFilterExpression(t, FilterNode{Kind: kind, Property: property})
			if _, err := ResolveFeatureFilter(expression, catalog); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, node := range []FilterNode{
		{Kind: FilterCompare, Property: "private", Operator: FilterEqual, Literal: testFilterLiteral(t, FilterNumber, "1")},
		{Kind: FilterCompare, Property: "n", Operator: FilterEqual, Literal: testFilterLiteral(t, FilterString, "1")},
	} {
		expression := testFilterExpression(t, node)
		_, err := ResolveFeatureFilter(expression, catalog)
		var invalid InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("resolution: %v", err)
		}
	}
	empty, err := NewFeatureQueryables(nil)
	if err != nil || empty.Validate() != nil {
		t.Fatal("explicit empty catalog invalid")
	}
	if (FeatureQueryables{}).Validate() == nil {
		t.Fatal("zero catalog valid")
	}
	expression := testFilterExpression(t, FilterNode{Kind: FilterBooleanConstant})
	if _, err := ResolveFeatureFilter(expression, empty); err != nil {
		t.Fatal(err)
	}

	node := FilterNode{Kind: FilterNot, Children: []FilterNode{{
		Kind: FilterCompare, Property: "n", Operator: FilterEqual,
		Literal: testFilterLiteral(t, FilterNumber, "1.5"),
	}}}
	owned := testFilterExpression(t, node)
	node.Children[0].Property = "private"
	returned := owned.Root()
	returned.Children[0].Property = "private"
	resolved, err := ResolveFeatureFilter(owned, catalog)
	if err != nil {
		t.Fatal(err)
	}
	detached := resolved.Expression().Root()
	detached.Children[0].Property = "private"
	detachedFields := resolved.Queryables().Fields()
	detachedFields[0].Name = "private"
	if resolved.Expression().Root().Children[0].Property != "n" {
		t.Fatal("expression snapshot mutated")
	}
	if !reflect.DeepEqual(resolved.Expression().Root(), owned.Clone().Root()) {
		t.Fatal("NOT nullable numeric expression simplified")
	}
	if _, ok := resolved.Queryables().Lookup("n"); !ok {
		t.Fatal("resolved catalog mutated")
	}
}

func TestFilterCatalogBudgetAndValidity(t *testing.T) {
	for _, fields := range [][]FeatureQueryable{
		{{Name: " ", Type: QueryableString}},
		{{Name: "a", Type: QueryableInvalid}},
		{{Name: "a", Type: QueryableString}, {Name: "a", Type: QueryableString}},
		{{Name: string([]byte{0xff}), Type: QueryableString}},
		{{Name: strings.Repeat("n", MaxQueryableNameBytes+1), Type: QueryableString}},
		make([]FeatureQueryable, MaxQueryableFields+1),
	} {
		if _, err := NewFeatureQueryables(fields); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
	fields := make([]FeatureQueryable, 65)
	for i := range fields {
		fields[i] = FeatureQueryable{Name: strings.Repeat("a", 1022) + strconv.Itoa(10+i), Type: QueryableString}
	}
	if _, err := NewFeatureQueryables(fields); err == nil {
		t.Fatal("catalog aggregate bytes ignored")
	}
	if _, err := NewFeatureQueryables(fields[:64]); err != nil {
		t.Fatalf("exact catalog byte boundary rejected: %v", err)
	}
	countBoundary := make([]FeatureQueryable, MaxQueryableFields)
	for i := range countBoundary {
		countBoundary[i] = FeatureQueryable{Name: "field" + strconv.Itoa(i), Type: QueryableString}
	}
	if _, err := NewFeatureQueryables(countBoundary); err != nil {
		t.Fatalf("exact catalog count boundary rejected: %v", err)
	}
	catalog, err := NewFeatureQueryables([]FeatureQueryable{{
		Name: strings.Repeat("n", MaxQueryableNameBytes), Type: QueryableString,
	}})
	if err != nil || catalog.Validate() != nil {
		t.Fatal("bounded catalog rejected")
	}
}

func TestFilterConcurrentDetachedSnapshots(t *testing.T) {
	catalog, err := NewFeatureQueryables([]FeatureQueryable{{Name: "n", Type: QueryableNumber, Nullable: true}})
	if err != nil {
		t.Fatal(err)
	}
	expression := testFilterExpression(t, FilterNode{
		Kind: FilterCompare, Property: "n", Operator: FilterEqual,
		Literal: testFilterLiteral(t, FilterNumber, "1.25"),
	})
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 50; j++ {
				resolved, err := ResolveFeatureFilter(expression, catalog)
				if err != nil {
					t.Error(err)
					return
				}
				root := resolved.Expression().Root()
				number, ok := root.Literal.Number()
				if !ok || number.Cmp(big.NewRat(5, 4)) != 0 {
					t.Error("exact number changed")
					return
				}
				number.SetInt64(0)
				fields := resolved.Queryables().Fields()
				fields[0].Name = "changed"
			}
		}()
	}
	workers.Wait()
}
