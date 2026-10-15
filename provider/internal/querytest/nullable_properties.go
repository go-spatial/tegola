package querytest

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// withPublicFieldDeclaration supplies literal fixture metadata, never backend
// output. It rejects inconsistent expected rows without rewriting their maps.
func withPublicFieldDeclaration(factory Factory) Factory {
	return func(t *testing.T, fixture Fixture) Instance {
		t.Helper()
		declared, err := declareFixturePublicFields(fixture)
		if err != nil {
			t.Fatal(err)
		}
		return factory(t, declared)
	}
}

func declareFixturePublicFields(fixture Fixture) (Fixture, error) {
	copy := cloneFixture(fixture)
	fields := map[string]bool{}
	for _, row := range copy.Rows {
		for field := range row.Feature.Tags {
			if strings.TrimSpace(field) == "" {
				return Fixture{}, fmt.Errorf("querytest: blank fixture public field")
			}
			fields[field] = true
		}
	}
	copy.PublicFields = make([]string, 0, len(fields))
	for field := range fields {
		copy.PublicFields = append(copy.PublicFields, field)
	}
	sort.Strings(copy.PublicFields)
	for _, row := range copy.Rows {
		if row.Metadata || row.MissingID || row.MalformedGeometry != "" {
			continue
		}
		if len(row.Feature.Tags) != len(copy.PublicFields) {
			return Fixture{}, fmt.Errorf("querytest: inconsistent fixture-wide public property keys")
		}
		for _, field := range copy.PublicFields {
			if _, ok := row.Feature.Tags[field]; !ok {
				return Fixture{}, fmt.Errorf("querytest: missing declared public property key")
			}
		}
	}
	return copy, nil
}

// RunNullableProfiles requires actual selected SQL NULL storage in both source
// profiles. Strict key presence distinguishes selected nil from unselected.
func RunNullableProfiles(t *testing.T, factory Factory, options ProfileOptions) {
	t.Helper()
	factory = withPublicFieldDeclaration(factory)
	for _, p := range []struct {
		name      string
		selection FixtureProfile
		options   Options
	}{
		{"ordinary nullable properties", OrdinaryTable, options.Ordinary},
		{"custom nullable properties", CustomSelection, options.Custom},
	} {
		t.Run(p.name, func(t *testing.T) {
			factory := withNativeRingOrientation(factory, p.options.NativeRingOrientationEquivalent)
			profile, err := copyTemporalPropertyProfile(p.options.PublicTemporalProperties)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range nullablePropertyCases(p.selection) {
				t.Run(c.name, func(t *testing.T) {
					if profile != nil {
						c.fixture, err = withPublicTemporalProperties(c.fixture, *profile)
						if err != nil {
							t.Fatal(err)
						}
					}
					instance := factory(t, cloneFixture(c.fixture))
					registerInstanceCleanup(t, instance)
					if issues, handled := checkSetup(instance, c); handled {
						for _, issue := range issues {
							t.Error(issue)
						}
						return
					}
					if instance.CountMode > OptionalCount {
						t.Fatal("invalid count mode")
					}
					for _, issue := range checkCase(instance, c) {
						t.Error(issue)
					}
					if c.mutateDelivery {
						for _, issue := range checkDimensionalConcurrent(instance, c) {
							t.Error(issue)
						}
					}
				})
			}
		})
	}
}

func nullablePropertyFixture(profile FixtureProfile) Fixture {
	fixture := Fixture{Profile: profile, PublicFields: []string{"name", "value"}, Rows: []Row{
		{Feature: provider.Feature{ID: 10, SRID: 4326, Geometry: geom.Point{0, 0}, Tags: map[string]any{"name": nil, "value": int64(0)}}},
		{Feature: provider.Feature{ID: 20, SRID: 4326, Geometry: geom.Point{1, 1}, Tags: map[string]any{"name": "", "value": nil}}},
		{Feature: provider.Feature{ID: 30, SRID: 4326, Geometry: geom.Point{2, 2}, Tags: map[string]any{"name": "nonempty", "value": int64(7)}}},
	}}
	if profile == CustomSelection {
		fixture.Rows = append(fixture.Rows, Row{ExcludedBySelection: true, Feature: provider.Feature{
			ID: 90, SRID: 4326, Geometry: geom.Point{1, 1}, Tags: map[string]any{"name": "excluded", "value": int64(90)},
		}})
	}
	return fixture
}

func nullablePropertyCases(profile FixtureProfile) []caseSpec {
	fixture := nullablePropertyFixture(profile)
	cases := []caseSpec{
		{name: "selected NULL zero and empty remain distinct", ids: []uint64{10, 20, 30}, matched: 3},
		{name: "selected name NULL survives value unselected", query: provider.FeatureQuery{Fields: []string{"name"}}, ids: []uint64{10, 20, 30}, matched: 3},
		{name: "selected value NULL survives name unselected", query: provider.FeatureQuery{Fields: []string{"value"}}, ids: []uint64{10, 20, 30}, matched: 3},
		{name: "nullable ID subset sorted", query: provider.FeatureQuery{IDs: []uint64{20, 10}}, ids: []uint64{10, 20}, matched: 2},
		{name: "nullable page first lookahead", query: provider.FeatureQuery{Limit: 1}, ids: []uint64{10}, matched: 3, more: true},
		{name: "nullable page middle selected NULL", query: provider.FeatureQuery{Limit: 1, Offset: 1, Fields: []string{"value"}}, ids: []uint64{20}, matched: 3, more: true},
		{name: "nullable page last", query: provider.FeatureQuery{Limit: 1, Offset: 2}, ids: []uint64{30}, matched: 3},
		{name: "nullable callback detached ownership", ids: []uint64{10, 20, 30}, matched: 3, mutateDelivery: true},
	}
	if profile == CustomSelection {
		cases = append(cases, caseSpec{name: "nullable custom excluded domain ID", query: provider.FeatureQuery{IDs: []uint64{90}}, ids: []uint64{}, matched: 0})
	}
	for i := range cases {
		cases[i].fixture = fixture
		if cases[i].query.Limit == 0 {
			cases[i].query.Limit = 100
		}
	}
	return cases
}
