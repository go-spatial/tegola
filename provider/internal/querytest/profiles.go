package querytest

import (
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

type FixtureProfile uint8

const (
	OrdinaryTable FixtureProfile = iota
	CustomSelection
)

type ProfileOptions struct {
	Ordinary, Custom Options
}

// RunProfiles materializes both real adapter paths. The factory must implement
// the declared custom SQL selection, including excluded stored rows. Profile is
// a test descriptor; it never relaxes a provider's catalog or SQL admission.
func RunProfiles(t *testing.T, factory Factory, options ProfileOptions) {
	t.Helper()
	t.Run("ordinary table", func(t *testing.T) {
		runProfile(t, factory, options.Ordinary, OrdinaryTable)
	})
	t.Run("custom selection", func(t *testing.T) {
		runProfile(t, factory, options.Custom, CustomSelection)
	})
}

func profileCases(profile FixtureProfile) []caseSpec {
	cases := contractCases()
	if profile == CustomSelection {
		cases = append(cases, customDomainCases()...)
	}
	for i := range cases {
		cases[i].fixture.Profile = profile
	}
	return cases
}

func customDomainFixture() Fixture {
	fixture := canonicalFixture()
	fixture.Profile = CustomSelection
	instant := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	fixture.Rows = append(fixture.Rows, Row{
		Feature: provider.Feature{
			ID: 90, SRID: 4326, Geometry: geom.Point{0, 0},
			Tags: map[string]any{"name": "excluded-domain", "value": int64(90)},
		},
		Start: &instant, End: cloneTime(&instant), ExcludedBySelection: true,
	})
	return fixture
}

func customDomainCases() []caseSpec {
	instant := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	fixture := customDomainFixture()
	return []caseSpec{
		{
			name: "custom domain unfiltered", fixture: fixture,
			query: provider.FeatureQuery{Limit: 100}, ids: []uint64{10, 20, 30, 40}, matched: 4,
		},
		{
			name: "custom excluded ID cannot escape domain", fixture: fixture,
			query: provider.FeatureQuery{Limit: 100, IDs: []uint64{90}}, ids: []uint64{}, matched: 0,
		},
		{
			name: "custom domain exact predicates before paging", fixture: fixture,
			query: provider.FeatureQuery{
				Limit: 1, Offset: 1, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326,
				Temporal: &provider.TemporalConstraint{Start: &instant, End: cloneTime(&instant)},
			},
			ids: []uint64{20}, matched: 2, more: false,
		},
	}
}
