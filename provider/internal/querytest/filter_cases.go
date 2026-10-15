package querytest

import (
	"strings"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

func filterFixture(profile FixtureProfile) Fixture {
	rows := []Row{
		{Feature: provider.Feature{ID: 10, Geometry: geom.Point{1, 0}, SRID: 4326, Tags: map[string]any{"n": nil, "s": nil, "b": nil}}},
		{Feature: provider.Feature{ID: 20, Geometry: geom.Point{2, 0}, SRID: 4326, Tags: map[string]any{"n": int64(0), "s": "", "b": false}}},
		{Feature: provider.Feature{ID: 30, Geometry: geom.Point{3, 0}, SRID: 4326, Tags: map[string]any{"n": int64(7), "s": "Alpha", "b": true}}},
		{Feature: provider.Feature{ID: 40, Geometry: geom.Point{4, 0}, SRID: 4326, Tags: map[string]any{"n": int64(-2), "s": "alpha", "b": false}}},
		{Feature: provider.Feature{ID: 50, Geometry: geom.Point{5, 0}, SRID: 4326, Tags: map[string]any{"n": int64(7), "s": "x' OR TRUE -- ;", "b": nil}}},
		{Feature: provider.Feature{ID: 60, Geometry: geom.Point{6, 0}, SRID: 4326, Tags: map[string]any{"n": int64(1), "s": "é", "b": true}}},
		{Feature: provider.Feature{ID: 70, Geometry: geom.Point{7, 0}, SRID: 4326, Tags: map[string]any{"n": int64(2), "s": "e\u0301", "b": false}}},
		{Feature: provider.Feature{ID: 80, Geometry: geom.Point{8, 0}, SRID: 4326, Tags: map[string]any{"n": int64(7), "s": "Alpha ", "b": true}}},
	}
	epoch := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	for i := range rows {
		if rows[i].Feature.ID == 30 || rows[i].Feature.ID == 80 {
			rows[i].Start = cloneTime(&epoch)
			rows[i].End = cloneTime(&epoch)
		}
		if rows[i].Feature.ID == 50 {
			later := epoch.Add(time.Second)
			rows[i].Start = &later
			rows[i].End = cloneTime(&later)
		}
	}
	if profile == CustomSelection {
		rows = append(rows, Row{Feature: provider.Feature{ID: 90, Geometry: geom.Point{9, 0}, SRID: 4326, Tags: map[string]any{"n": int64(7), "s": "Alpha", "b": true}}, ExcludedBySelection: true})
	}
	return Fixture{Profile: profile, Rows: rows, TemporalStorage: UnixSeconds, FilterFields: []provider.FeatureQueryable{
		{Name: "b", Type: provider.QueryableBoolean, Nullable: true},
		{Name: "n", Type: provider.QueryableInteger, Nullable: true},
		{Name: "s", Type: provider.QueryableString, Nullable: true},
	}}
}

type filterBuilder struct{ err error }

func (b *filterBuilder) compare(property string, op provider.FilterCompareOperator, kind provider.FilterScalarType, text string) provider.FilterNode {
	literal, err := provider.NewFilterLiteral(kind, text)
	if err != nil && b.err == nil {
		b.err = err
	}
	return provider.FilterNode{Kind: provider.FilterCompare, Operator: op, Property: property, Literal: literal}
}
func (b *filterBuilder) expression(node provider.FilterNode) *provider.FilterExpression {
	value, err := provider.NewFilterExpression(node)
	if err != nil && b.err == nil {
		b.err = err
	}
	return &value
}
func filterCases(profile FixtureProfile) ([]caseSpec, error) {
	fixture := filterFixture(profile)
	b := filterBuilder{}
	num := func(op provider.FilterCompareOperator, text string) provider.FilterNode {
		return b.compare("n", op, provider.FilterNumber, text)
	}
	str := func(op provider.FilterCompareOperator, text string) provider.FilterNode {
		return b.compare("s", op, provider.FilterString, text)
	}
	boolean := func(value string) provider.FilterNode {
		return b.compare("b", provider.FilterEqual, provider.FilterBoolean, value)
	}
	negate := func(child provider.FilterNode) provider.FilterNode {
		return provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{child}}
	}
	logical := func(kind provider.FilterKind, left, right provider.FilterNode) provider.FilterNode {
		return provider.FilterNode{Kind: kind, Children: []provider.FilterNode{left, right}}
	}
	null := func(property string) provider.FilterNode {
		return provider.FilterNode{Kind: provider.FilterIsNull, Property: property}
	}
	specifications := []struct {
		name string
		node provider.FilterNode
		ids  []uint64
	}{
		{name: "boolean TRUE full domain", node: provider.FilterNode{Kind: provider.FilterBooleanConstant, Boolean: true}, ids: []uint64{10, 20, 30, 40, 50, 60, 70, 80}},
		{name: "boolean FALSE known zero", node: provider.FilterNode{Kind: provider.FilterBooleanConstant}, ids: []uint64{}},
		{name: "numeric NULL", node: null("n"), ids: []uint64{10}},
		{name: "numeric NOT NULL", node: provider.FilterNode{Kind: provider.FilterIsNotNull, Property: "n"}, ids: []uint64{20, 30, 40, 50, 60, 70, 80}},
		{name: "string NULL", node: null("s"), ids: []uint64{10}},
		{name: "boolean NULL", node: null("b"), ids: []uint64{10, 50}},
		{name: "numeric equal", node: num(provider.FilterEqual, "7"), ids: []uint64{30, 50, 80}},
		{name: "numeric not equal excludes NULL", node: num(provider.FilterNotEqual, "7"), ids: []uint64{20, 40, 60, 70}},
		{name: "numeric less", node: num(provider.FilterLess, "1"), ids: []uint64{20, 40}},
		{name: "numeric less equal", node: num(provider.FilterLessEqual, "1"), ids: []uint64{20, 40, 60}},
		{name: "numeric greater", node: num(provider.FilterGreater, "1"), ids: []uint64{30, 50, 70, 80}},
		{name: "numeric greater equal", node: num(provider.FilterGreaterEqual, "1"), ids: []uint64{30, 50, 60, 70, 80}},
		{name: "numeric equivalent scientific", node: num(provider.FilterEqual, "+7.000e0"), ids: []uint64{30, 50, 80}},
		{name: "numeric exact fraction no flooring", node: num(provider.FilterEqual, "1.25"), ids: []uint64{}},
		{name: "numeric fractional lower boundary", node: num(provider.FilterLessEqual, "1.25"), ids: []uint64{20, 40, 60}},
		{name: "numeric fractional upper boundary", node: num(provider.FilterGreater, "1.25"), ids: []uint64{30, 50, 70, 80}},
		{name: "numeric above int64 range", node: num(provider.FilterGreater, "9223372036854775808"), ids: []uint64{}},
		{name: "NOT impossible numeric preserves NULL", node: negate(num(provider.FilterGreater, "9223372036854775808")), ids: []uint64{20, 30, 40, 50, 60, 70, 80}},
		{name: "numeric below int64 range", node: num(provider.FilterLess, "-9223372036854775809"), ids: []uint64{}},
		{name: "NOT below range preserves NULL", node: negate(num(provider.FilterLess, "-9223372036854775809")), ids: []uint64{20, 30, 40, 50, 60, 70, 80}},
		{name: "NOT comparison preserves UNKNOWN", node: negate(num(provider.FilterEqual, "7")), ids: []uint64{20, 40, 60, 70}},
		{name: "OR includes NULL explicitly", node: logical(provider.FilterOr, num(provider.FilterEqual, "7"), null("n")), ids: []uint64{10, 30, 50, 80}},
		{name: "AND boolean nullable", node: logical(provider.FilterAnd, num(provider.FilterEqual, "7"), boolean("true")), ids: []uint64{30, 80}},
		{name: "grouped OR AND NULL", node: logical(provider.FilterAnd, logical(provider.FilterOr, num(provider.FilterEqual, "7"), null("n")), null("b")), ids: []uint64{10, 50}},
		{name: "boolean true exact", node: boolean("true"), ids: []uint64{30, 60, 80}},
		{name: "boolean false exact", node: boolean("false"), ids: []uint64{20, 40, 70}},
		{name: "boolean NOT false UNKNOWN", node: negate(boolean("false")), ids: []uint64{30, 60, 80}},
		{name: "boolean not equal", node: b.compare("b", provider.FilterNotEqual, provider.FilterBoolean, "true"), ids: []uint64{20, 40, 70}},
		{name: "boolean less", node: b.compare("b", provider.FilterLess, provider.FilterBoolean, "true"), ids: []uint64{20, 40, 70}},
		{name: "boolean less equal", node: b.compare("b", provider.FilterLessEqual, provider.FilterBoolean, "false"), ids: []uint64{20, 40, 70}},
		{name: "boolean greater", node: b.compare("b", provider.FilterGreater, provider.FilterBoolean, "false"), ids: []uint64{30, 60, 80}},
		{name: "boolean greater equal", node: b.compare("b", provider.FilterGreaterEqual, provider.FilterBoolean, "true"), ids: []uint64{30, 60, 80}},
		{name: "boolean NOT NULL", node: provider.FilterNode{Kind: provider.FilterIsNotNull, Property: "b"}, ids: []uint64{20, 30, 40, 60, 70, 80}},
		{name: "string NOT NULL", node: provider.FilterNode{Kind: provider.FilterIsNotNull, Property: "s"}, ids: []uint64{20, 30, 40, 50, 60, 70, 80}},

		{name: "string equal case exact", node: str(provider.FilterEqual, "Alpha"), ids: []uint64{30}},
		{name: "string not equal NULL excluded", node: str(provider.FilterNotEqual, "Alpha"), ids: []uint64{20, 40, 50, 60, 70, 80}},
		{name: "string less bytewise", node: str(provider.FilterLess, "Alpha"), ids: []uint64{20}},
		{name: "string less equal bytewise", node: str(provider.FilterLessEqual, "Alpha"), ids: []uint64{20, 30}},
		{name: "string greater bytewise", node: str(provider.FilterGreater, "Alpha"), ids: []uint64{40, 50, 60, 70, 80}},
		{name: "string greater equal bytewise", node: str(provider.FilterGreaterEqual, "Alpha"), ids: []uint64{30, 40, 50, 60, 70, 80}},
		{name: "string lowercase distinct", node: str(provider.FilterEqual, "alpha"), ids: []uint64{40}},
		{name: "string empty distinct NULL", node: str(provider.FilterEqual, ""), ids: []uint64{20}},
		{name: "string trailing space retained", node: str(provider.FilterEqual, "Alpha "), ids: []uint64{80}},
		{name: "string composed accent", node: str(provider.FilterEqual, "é"), ids: []uint64{60}},
		{name: "string decomposed accent", node: str(provider.FilterEqual, "e\u0301"), ids: []uint64{70}},
		{name: "injection literal bound only", node: str(provider.FilterEqual, "x' OR TRUE -- ;"), ids: []uint64{50}},
		{name: "long string bound without truncation", node: str(provider.FilterEqual, "Alpha"+strings.Repeat("x", 16379)), ids: []uint64{}},
		{name: "NUL request binary bound without truncation", node: str(provider.FilterEqual, "Alpha\x00suffix"), ids: []uint64{}},
	}
	cases := []caseSpec{}
	for _, spec := range specifications {
		cases = append(cases, caseSpec{name: spec.name, fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: b.expression(spec.node)}, ids: spec.ids, matched: uint64(len(spec.ids))})
	}
	cases = append(cases,
		caseSpec{name: "string type mismatch before callbacks", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: b.expression(b.compare("s", provider.FilterEqual, provider.FilterNumber, "7"))}, invalidQuery: true},
		caseSpec{name: "boolean type mismatch before callbacks", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: b.expression(b.compare("b", provider.FilterEqual, provider.FilterNumber, "1"))}, invalidQuery: true},
	)
	equalSeven := b.expression(num(provider.FilterEqual, "7"))
	for _, page := range []struct {
		name   string
		offset uint64
		ids    []uint64
		more   bool
	}{
		{name: "filter first page lookahead", offset: 0, ids: []uint64{30}, more: true},
		{name: "filter middle page lookahead", offset: 1, ids: []uint64{50}, more: true},
		{name: "filter last page stable", offset: 2, ids: []uint64{80}},
	} {
		cases = append(cases, caseSpec{name: page.name, fixture: fixture, query: provider.FeatureQuery{Limit: 1, Offset: page.offset, Filter: equalSeven}, ids: page.ids, matched: 3, more: page.more})
	}
	epoch := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	cases = append(cases,
		caseSpec{name: "filter bbox datetime and selected field", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven, BoundsSRID: 4326, Bounds: []geom.Extent{{2, -1, 6, 1}}, Temporal: &provider.TemporalConstraint{Start: &epoch, End: &epoch}, Fields: []string{"s"}}, ids: []uint64{30}, matched: 1},
		caseSpec{name: "filter projection excludes predicate field", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven, Fields: []string{"s"}}, ids: []uint64{30, 50, 80}, matched: 3},
		caseSpec{name: "filter identity selector composed", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven, IDs: []uint64{80, 30}}, ids: []uint64{30, 80}, matched: 2},
		caseSpec{name: "filter detached ownership and concurrency", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven}, ids: []uint64{30, 50, 80}, matched: 3, mutateDelivery: true},
		caseSpec{name: "filter precanceled", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven}, preCancel: true},
		caseSpec{name: "filter deadline", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven}, deadline: true},
		caseSpec{name: "filter mid callback cancellation", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven}, cancelAfterFirst: true},
		caseSpec{name: "filter callback error chain", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: equalSeven}, callbackFailure: true},
	)
	for _, alias := range []string{"id", "geom", "source_n", "selection_flag", "unknown"} {
		cases = append(cases, caseSpec{name: "unpublished filter alias " + alias, fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: b.expression(b.compare(alias, provider.FilterEqual, provider.FilterNumber, "1"))}, invalidQuery: true})
	}
	cases = append(cases, caseSpec{name: "filter type mismatch before callbacks", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: b.expression(b.compare("n", provider.FilterEqual, provider.FilterString, "7"))}, invalidQuery: true})
	if profile == CustomSelection {
		cases = append(cases, caseSpec{name: "custom domain excluded ID90 filter TRUE", fixture: fixture, query: provider.FeatureQuery{Limit: 100, Filter: b.expression(provider.FilterNode{Kind: provider.FilterBooleanConstant, Boolean: true}), IDs: []uint64{90}}, ids: []uint64{}, matched: 0})
	}
	truthCases, err := filterTruthTableCases(profile)
	if err != nil {
		return nil, err
	}
	cases = append(cases, truthCases...)
	return cases, b.err
}
