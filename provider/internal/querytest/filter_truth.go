package querytest

import (
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// The literal truth matrix is independent of every provider/evaluator/compiler.
// NULL is the UNKNOWN input/result. Only TRUE results are delivered by a filter.
func filterTruthTableCases(profile FixtureProfile) ([]caseSpec, error) {
	pairs := []struct {
		id                     uint64
		n, b                   any
		and, or, notAnd, notOr string
	}{
		{id: 10, n: int64(7), b: true, and: "T", or: "T", notAnd: "F", notOr: "F"},
		{id: 20, n: int64(7), b: false, and: "F", or: "T", notAnd: "T", notOr: "F"},
		{id: 30, n: int64(7), b: nil, and: "U", or: "T", notAnd: "U", notOr: "F"},
		{id: 40, n: int64(0), b: true, and: "F", or: "T", notAnd: "T", notOr: "F"},
		{id: 50, n: int64(0), b: false, and: "F", or: "F", notAnd: "T", notOr: "T"},
		{id: 60, n: int64(0), b: nil, and: "F", or: "U", notAnd: "T", notOr: "U"},
		{id: 70, n: nil, b: true, and: "U", or: "T", notAnd: "U", notOr: "F"},
		{id: 80, n: nil, b: false, and: "F", or: "U", notAnd: "T", notOr: "U"},
		{id: 100, n: nil, b: nil, and: "U", or: "U", notAnd: "U", notOr: "U"},
	}
	fixture := filterFixture(profile)
	fixture.Rows = []Row{}
	for _, pair := range pairs {
		fixture.Rows = append(fixture.Rows, Row{Feature: provider.Feature{ID: pair.id, Geometry: geom.Point{0, 0}, SRID: 4326, Tags: map[string]any{"n": pair.n, "s": "truth-table", "b": pair.b}}})
	}
	if profile == CustomSelection {
		fixture.Rows = append(fixture.Rows, Row{Feature: provider.Feature{ID: 90, Geometry: geom.Point{0, 0}, SRID: 4326, Tags: map[string]any{"n": int64(7), "s": "excluded", "b": true}}, ExcludedBySelection: true})
	}
	builder := filterBuilder{}
	p := builder.compare("n", provider.FilterEqual, provider.FilterNumber, "7")
	q := builder.compare("b", provider.FilterEqual, provider.FilterBoolean, "true")
	and := provider.FilterNode{Kind: provider.FilterAnd, Children: []provider.FilterNode{p, q}}
	or := provider.FilterNode{Kind: provider.FilterOr, Children: []provider.FilterNode{p, q}}
	notAnd := provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{and}}
	notOr := provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{or}}
	cases := []caseSpec{}
	for _, pair := range pairs {
		for _, operation := range []struct {
			name  string
			node  provider.FilterNode
			truth string
		}{
			{name: "AND", node: and, truth: pair.and}, {name: "OR", node: or, truth: pair.or},
			{name: "NOT AND", node: notAnd, truth: pair.notAnd}, {name: "NOT OR", node: notOr, truth: pair.notOr},
		} {
			ids := []uint64{}
			if operation.truth == "T" {
				ids = []uint64{pair.id}
			}
			// IDs select one fixed row; the truth table, not the AST, supplies membership.
			cases = append(cases, caseSpec{name: "3VL " + operation.name + " row " + truthRowName(pair.id), fixture: fixture, query: provider.FeatureQuery{Limit: 100, IDs: []uint64{pair.id}, Filter: builder.expression(operation.node)}, ids: ids, matched: uint64(len(ids))})
		}
	}
	return cases, builder.err
}
func truthRowName(id uint64) string {
	switch id {
	case 10:
		return "TRUE TRUE"
	case 20:
		return "TRUE FALSE"
	case 30:
		return "TRUE UNKNOWN"
	case 40:
		return "FALSE TRUE"
	case 50:
		return "FALSE FALSE"
	case 60:
		return "FALSE UNKNOWN"
	case 70:
		return "UNKNOWN TRUE"
	case 80:
		return "UNKNOWN FALSE"
	default:
		return "UNKNOWN UNKNOWN"
	}
}
