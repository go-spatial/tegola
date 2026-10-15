package querytest

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

// RunFilterWithOptions checks only the ordinary admitted filter domain.
func RunFilterWithOptions(t *testing.T, factory Factory, options Options) {
	t.Helper()
	runFilterProfile(t, factory, options, OrdinaryTable)
}

// RunFilterProfiles checks actual ordinary and configured custom selection paths.
// Backends without admitted custom selection must use RunFilterWithOptions and
// separately prove their unsupported boundary, not substitute ordinary storage.
func RunFilterProfiles(t *testing.T, factory Factory, options ProfileOptions) {
	t.Helper()
	t.Run("ordinary filters", func(t *testing.T) { runFilterProfile(t, factory, options.Ordinary, OrdinaryTable) })
	t.Run("custom filters", func(t *testing.T) { runFilterProfile(t, factory, options.Custom, CustomSelection) })
}
func runFilterProfile(t *testing.T, factory Factory, options Options, selection FixtureProfile) {
	t.Helper()
	factory = withPublicFieldDeclaration(factory)
	cases, err := filterCases(selection)
	if err != nil {
		t.Fatal(err)
	}
	properties, err := copyTemporalPropertyProfile(options.PublicTemporalProperties)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if properties != nil {
				fixture, err := withFilterTemporalProperties(c.fixture, *properties)
				if err != nil {
					t.Fatal(err)
				}
				c.fixture = fixture
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
			for _, issue := range checkFilterCatalog(instance, c.fixture) {
				t.Error(issue)
			}
			for _, issue := range checkCase(instance, c) {
				t.Error(issue)
			}
			if c.invalidQuery {
				for _, issue := range checkFilterClientError(instance, c) {
					t.Error(issue)
				}
			}
			if c.mutateDelivery {
				for _, issue := range checkDimensionalConcurrent(instance, c) {
					t.Error(issue)
				}
			}
		})
	}
}
func checkFilterCatalog(instance Instance, fixture Fixture) []string {
	if instance.QueryableLayer == nil || (reflect.ValueOf(instance.QueryableLayer).Kind() == reflect.Pointer && reflect.ValueOf(instance.QueryableLayer).IsNil()) {
		return []string{"actual queryable layer absent"}
	}
	actual, err := instance.QueryableLayer.FeatureQueryables()
	if err != nil {
		return []string{"actual queryables unavailable"}
	}
	if err := actual.Validate(); err != nil {
		return []string{"actual queryables invalid"}
	}
	expected, err := provider.NewFeatureQueryables(fixture.FilterFields)
	if err != nil {
		return []string{"static filter schema invalid"}
	}
	if !reflect.DeepEqual(actual.Fields(), expected.Fields()) {
		return []string{"actual queryables differ from static public filter schema"}
	}
	detached := actual.Fields()
	if len(detached) > 0 {
		detached[0].Name = "querytest-mutated-catalog"
		repeated, err := instance.QueryableLayer.FeatureQueryables()
		if err != nil || !reflect.DeepEqual(repeated.Fields(), expected.Fields()) {
			return []string{"queryable metadata retained mutable caller values"}
		}
	}
	return nil
}
func checkFilterClientError(instance Instance, c caseSpec) []string {
	var callbacks atomic.Int32
	_, err, _ := observeQuery(context.Background(), instance.Querier, instance.Layer, cloneQuery(c.query), func(*provider.Feature) error { callbacks.Add(1); return nil })
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) || invalid.Field != "filter" || callbacks.Load() != 0 {
		return []string{"invalid filter must have filter field and no callbacks"}
	}
	return nil
}
