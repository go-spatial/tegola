package querytest

import (
	"context"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestCustomProfileDescriptorsAndOrdinaryCompatibility(t *testing.T) {
	ordinary := profileCases(OrdinaryTable)
	if !reflect.DeepEqual(ordinary, contractCases()) {
		t.Fatal("ordinary default contract changed")
	}
	custom := profileCases(CustomSelection)
	if len(custom) != len(ordinary)+3 {
		t.Fatal("custom domain cases missing")
	}
	for _, c := range custom {
		if c.fixture.Profile != CustomSelection {
			t.Fatal("factory profile not delivered")
		}
		if c.name == "malformed explicit native" &&
			(len(c.fixture.Rows) != 1 || c.fixture.Rows[0].MalformedGeometry != "native") {
			t.Fatal("native malformed fixture changed")
		}
	}
	fixture := customDomainFixture()
	before := cloneFixture(fixture)
	copy := cloneFixture(fixture)
	if copy.Profile != CustomSelection || !copy.Rows[4].ExcludedBySelection || copy.Rows[4].Feature.ID != 90 {
		t.Fatal("domain descriptor lost")
	}
	copy.Rows[4].Feature.Tags["name"] = "mutated"
	copy.Rows[4].ExcludedBySelection = false
	if !reflect.DeepEqual(fixture, before) {
		t.Fatal("domain descriptor shares mutable fixture")
	}
}

func staticDomainDelivery(c caseSpec, ids []uint64) []provider.Feature {
	features := []provider.Feature{}
	for _, id := range ids {
		for _, row := range c.fixture.Rows {
			if row.Feature.ID == id {
				features = append(features, cloneFeature(row.Feature))
			}
		}
	}
	return features
}

// Fixed selection lists prove checker mechanics. No SQL implementation or
// production predicate supplies the expected IDs or counts.
func TestCustomDomainCheckerPositiveAndIgnoredSelectionControls(t *testing.T) {
	for _, c := range customDomainCases() {
		t.Run(c.name, func(t *testing.T) {
			good := staticDomainDelivery(c, c.ids)
			count := c.matched
			result := provider.FeatureQueryResult{
				NumberReturned: uint64(len(good)), NumberMatched: &count, HasMore: c.more,
			}
			if issues := checkResponse(result, good, c, ExactCount); len(issues) != 0 {
				t.Fatal(issues)
			}
			bad := staticDomainDelivery(c, []uint64{90})
			wrongCount := c.matched + 1
			wrong := provider.FeatureQueryResult{
				NumberReturned: uint64(len(bad)), NumberMatched: &wrongCount, HasMore: true,
			}
			if len(checkResponse(wrong, bad, c, ExactCount)) == 0 {
				t.Fatal("excluded custom domain ID accepted")
			}
			if c.name == "custom domain exact predicates before paging" {
				wrong.NumberReturned = uint64(len(good))
				wrong.NumberMatched = nil
				if len(checkResponse(wrong, good, c, UnknownCount)) == 0 {
					t.Fatal("ignored domain changed lookahead but passed unknown-count checker")
				}
			}
		})
	}
}

func TestCustomProfileConcurrentSelectionContext(t *testing.T) {
	fixture := customDomainFixture()
	querier := brokenQuerier{run: func(
		_ context.Context,
		q provider.FeatureQuery,
		fn func(*provider.Feature) error,
	) (provider.FeatureQueryResult, error) {
		if !reflect.DeepEqual(q.IDs, []uint64{30, 10, 20, 90}) {
			t.Error("concurrent custom selection omitted excluded ID")
		}
		for _, feature := range staticDomainDelivery(caseSpec{fixture: fixture}, []uint64{10, 20}) {
			if err := fn(&feature); err != nil {
				return provider.FeatureQueryResult{}, err
			}
		}
		count := uint64(3)
		return provider.FeatureQueryResult{NumberReturned: 2, NumberMatched: &count, HasMore: true}, nil
	}}
	if issues := checkConcurrentWithFixture(Instance{Querier: querier, CountMode: ExactCount}, fixture); len(issues) != 0 {
		t.Fatal("concurrent profile checker", issues)
	}
}
