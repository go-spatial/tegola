package querytest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

type filterReferenceLayer struct{ fields []provider.FeatureQueryable }

func (l filterReferenceLayer) FeatureQueryables() (provider.FeatureQueryables, error) {
	return provider.NewFeatureQueryables(l.fields)
}

// This fixed-list adapter checks harness mechanics. It never evaluates an AST,
// runs a database or establishes backend truth/collation/type parity.
func filterControl(c caseSpec, change func(*provider.Feature), ids []uint64, mutateInput bool) Instance {
	if ids == nil {
		ids = c.ids
	}
	if c.cancelAfterFirst || c.callbackFailure {
		ids = []uint64{30, 50, 80}
	}
	delivery := staticDomainDelivery(c, ids)
	return Instance{QueryableLayer: filterReferenceLayer{fields: append([]provider.FeatureQueryable(nil), c.fixture.FilterFields...)}, CountMode: ExactCount, Querier: brokenQuerier{run: func(ctx context.Context, q provider.FeatureQuery, callback func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if err := ctx.Err(); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		if c.invalidQuery {
			return provider.FeatureQueryResult{}, provider.InvalidFeatureQueryError{Field: "filter", Reason: "reference invalid property/type"}
		}
		if mutateInput && q.Filter != nil {
			*q.Filter = provider.FilterExpression{}
		}
		for _, stored := range delivery {
			if err := ctx.Err(); err != nil {
				return provider.FeatureQueryResult{}, err
			}
			feature := cloneFeature(stored)
			if len(c.query.Fields) > 0 {
				feature.Tags = map[string]any{"s": feature.Tags["s"]}
			}
			if change != nil {
				change(&feature)
			}
			if err := callback(&feature); err != nil {
				return provider.FeatureQueryResult{}, err
			}
		}
		if err := ctx.Err(); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		count := c.matched
		return provider.FeatureQueryResult{NumberReturned: uint64(len(delivery)), NumberMatched: &count, HasMore: c.more}, nil
	}}}
}
func findFilterCase(t *testing.T, profile FixtureProfile, name string) caseSpec {
	t.Helper()
	cases, err := filterCases(profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.name == name {
			return c
		}
	}
	t.Fatal("static filter case absent")
	return caseSpec{}
}
func TestFilterAllStaticReferenceCases(t *testing.T) {
	for _, profile := range []FixtureProfile{OrdinaryTable, CustomSelection} {
		cases, err := filterCases(profile)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				instance := filterControl(c, nil, nil, false)
				if issues := checkFilterCatalog(instance, c.fixture); len(issues) != 0 {
					t.Fatal(issues)
				}
				if issues := checkCase(instance, c); len(issues) != 0 {
					t.Fatal(issues)
				}
				if c.invalidQuery {
					if issues := checkFilterClientError(instance, c); len(issues) != 0 {
						t.Fatal(issues)
					}
				}
			})
		}
	}
}
func TestFilterNegativeMembershipAndPagingControls(t *testing.T) {
	cases := []struct {
		name   string
		target string
		ids    []uint64
	}{
		{name: "UNKNOWN collapsed before NOT", target: "NOT comparison preserves UNKNOWN", ids: []uint64{10, 20, 40, 60, 70}},
		{name: "fraction floored", target: "numeric exact fraction no flooring", ids: []uint64{60}},
		{name: "overflow fold loses NULL", target: "NOT impossible numeric preserves NULL", ids: []uint64{10, 20, 30, 40, 50, 60, 70, 80}},
		{name: "casefold", target: "string equal case exact", ids: []uint64{30, 40}},
		{name: "trailing trim", target: "string equal case exact", ids: []uint64{30, 80}},
		{name: "accent normalize", target: "string composed accent", ids: []uint64{60, 70}},
		{name: "long literal truncate", target: "long string bound without truncation", ids: []uint64{30}},
		{name: "NUL literal truncate", target: "NUL request binary bound without truncation", ids: []uint64{30}},
		{name: "SQL value injection", target: "injection literal bound only", ids: []uint64{10, 20, 30, 40, 50, 60, 70, 80}},
		{name: "domain escaped", target: "numeric equal", ids: []uint64{30, 50, 80, 90}},
		{name: "filter after offset", target: "filter middle page lookahead", ids: []uint64{30}},
		{name: "logical grouping lost", target: "grouped OR AND NULL", ids: []uint64{10, 30, 50, 80}},
		{name: "UNKNOWN TRUE collapsed under NOT AND", target: "3VL NOT AND row UNKNOWN TRUE", ids: []uint64{70}},
		{name: "FALSE UNKNOWN collapsed under NOT OR", target: "3VL NOT OR row FALSE UNKNOWN", ids: []uint64{60}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := findFilterCase(t, CustomSelection, tc.target)
			if len(checkCase(filterControl(c, nil, tc.ids, false), c)) == 0 {
				t.Fatal("bad implementation passed fixed oracle")
			}
		})
	}
}
func TestFilterPropertyAndInputNegativeControls(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "boolean TRUE full domain")
	for _, change := range []func(*provider.Feature){
		func(f *provider.Feature) {
			for k, v := range f.Tags {
				if v == nil {
					delete(f.Tags, k)
				}
			}
		},
		func(f *provider.Feature) {
			if value, ok := f.Tags["b"].(bool); ok {
				if value {
					f.Tags["b"] = int64(1)
				} else {
					f.Tags["b"] = int64(0)
				}
			}
		},
	} {
		if len(checkCase(filterControl(c, change, nil, false), c)) == 0 {
			t.Fatal("NULL omission or bool coercion passed")
		}
	}
	selected := findFilterCase(t, OrdinaryTable, "filter projection excludes predicate field")
	if len(checkCase(filterControl(selected, func(f *provider.Feature) { f.Tags["n"] = int64(7) }, nil, false), selected)) == 0 {
		t.Fatal("unselected predicate field exposed")
	}
	if len(checkCase(filterControl(c, nil, nil, true), c)) == 0 {
		t.Fatal("filter pointer overwrite not detected")
	}
	if err := c.query.Filter.Validate(); err != nil {
		t.Fatal("negativecontrol contaminated canonical expression", err)
	}
	invalid := findFilterCase(t, OrdinaryTable, "unpublished filter alias id")
	bad := filterControl(invalid, nil, nil, false)
	bad.Querier = brokenQuerier{run: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		return provider.FeatureQueryResult{}, provider.InvalidFeatureQueryError{Field: "other", Reason: "reference wrong classification"}
	}}
	if len(checkFilterClientError(bad, invalid)) == 0 {
		t.Fatal("wrong invalid field accepted")
	}
}
func TestFilterCatalogAndFixtureDetachedControls(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "numeric equal")
	instance := filterControl(c, nil, nil, false)
	instance.QueryableLayer = nil
	if len(checkFilterCatalog(instance, c.fixture)) == 0 {
		t.Fatal("missing metadata accepted")
	}
	fields := append([]provider.FeatureQueryable(nil), c.fixture.FilterFields...)
	fields = append(fields, provider.FeatureQueryable{Name: "id", Type: provider.QueryableInteger})
	instance.QueryableLayer = filterReferenceLayer{fields: fields}
	if len(checkFilterCatalog(instance, c.fixture)) == 0 {
		t.Fatal("private identity inferred as queryable")
	}
	fields = c.fixture.FilterFields[:2]
	instance.QueryableLayer = filterReferenceLayer{fields: fields}
	if len(checkFilterCatalog(instance, c.fixture)) == 0 {
		t.Fatal("missing logical field accepted")
	}
	clone := cloneFixture(c.fixture)
	clone.FilterFields[0].Name = "changed"
	if c.fixture.FilterFields[0].Name != "b" {
		t.Fatal("fixture retained metadata backing array")
	}
	query := cloneQuery(c.query)
	*query.Filter = provider.FilterExpression{}
	if err := c.query.Filter.Validate(); err != nil {
		t.Fatal("query snapshot retains expression pointer")
	}
	for _, profile := range []FixtureProfile{OrdinaryTable, CustomSelection} {
		fixture := filterFixture(profile)
		declared, err := declareFixturePublicFields(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(declared.PublicFields, []string{"b", "n", "s"}) {
			t.Fatal("fixture schema not exact")
		}
		if profile == CustomSelection && (!declared.Rows[len(declared.Rows)-1].ExcludedBySelection || declared.Rows[len(declared.Rows)-1].Feature.ID != 90) {
			t.Fatal("excluded source row lost")
		}
	}
}
func TestFilterCountKnownZeroAndUnknownControls(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "boolean FALSE known zero")
	instance := filterControl(c, nil, nil, false)
	instance.Querier = brokenQuerier{run: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		return provider.FeatureQueryResult{}, nil
	}}
	if len(checkCase(instance, c)) == 0 {
		t.Fatal("exact count silently unknown")
	}
	instance.CountMode = UnknownCount
	if issues := checkCase(instance, c); len(issues) != 0 {
		t.Fatal("declared unknown count rejected", issues)
	}
}
func TestFilterConcurrentMetadataAndDelivery(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "filter detached ownership and concurrency")
	if issues := checkDimensionalConcurrent(filterControl(c, nil, nil, false), c); len(issues) != 0 {
		t.Fatal(issues)
	}
}
func TestFilterPublicTemporalUnitOverride(t *testing.T) {
	fixture := filterFixture(CustomSelection)
	p := TemporalPropertyProfile{StartField: "start_time", EndField: "end_time", Storage: fixture.TemporalStorage}
	actual, err := withFilterTemporalProperties(fixture, p)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := declareFixturePublicFields(actual)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(declared.PublicFields, []string{"b", "end_time", "n", "s", "start_time"}) {
		t.Fatal("explicit public columns lost")
	}
	if declared.Rows[2].Feature.Tags["start_time"] != int64(1790899200) {
		t.Fatal("public time not static declared epoch seconds")
	}
}
func TestFilterReferenceCallbackChainNegative(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "filter callback error chain")
	instance := filterControl(c, nil, nil, false)
	original := instance.Querier
	instance.Querier = brokenQuerier{run: func(ctx context.Context, q provider.FeatureQuery, callback func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		result, err := original.QueryFeatures(ctx, "", q, callback)
		if err != nil {
			return result, errors.New("reference lost cause")
		}
		return result, nil
	}}
	if len(checkCase(instance, c)) == 0 {
		t.Fatal("lost callback chain accepted")
	}
}

func TestFilterPublicTemporalCatalogNegativeControls(t *testing.T) {
	fixture := filterFixture(CustomSelection)
	profile := TemporalPropertyProfile{StartField: "start_time", EndField: "end_time", Storage: UnixNanoseconds}
	actual, err := withFilterTemporalProperties(fixture, profile)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Rows[2].Feature.Tags["start_time"] != int64(1790899200) {
		t.Fatal("suite failed declared seconds override")
	}
	if actual.Rows[0].Feature.Tags["start_time"] != nil || actual.Rows[0].Feature.Tags["end_time"] != nil {
		t.Fatal("projected NULL temporal aliases absent or fabricated")
	}
	instance := Instance{QueryableLayer: filterReferenceLayer{fields: fixture.FilterFields}}
	if len(checkFilterCatalog(instance, actual)) == 0 {
		t.Fatal("missing public temporal queryables accepted")
	}
	instance.QueryableLayer = filterReferenceLayer{fields: actual.FilterFields}
	if issues := checkFilterCatalog(instance, actual); len(issues) != 0 {
		t.Fatal(issues)
	}
	for _, alias := range []string{"n", "s", "b", "start_time"} {
		collision := profile
		collision.StartField = alias
		if alias == "start_time" {
			collision.EndField = alias
		}
		if _, err := withFilterTemporalProperties(fixture, collision); err == nil {
			t.Fatal("public temporal alias collision accepted")
		}
	}
	c := findFilterCase(t, CustomSelection, "boolean TRUE full domain")
	c.fixture = actual
	if len(checkCase(filterControl(c, func(f *provider.Feature) {
		if f.Tags["start_time"] != nil {
			f.Tags["start_time"] = int64(1790899200000000000)
		}
	}, nil, false), c)) == 0 {
		t.Fatal("wrong public source unit accepted")
	}
	if len(fixture.FilterFields) != 3 || len(fixture.Rows[0].Feature.Tags) != 3 {
		t.Fatal("private fixture mutated by public profile")
	}
}

func TestFilterCancellationNegativeControl(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "filter precanceled")
	instance := filterControl(c, nil, nil, false)
	instance.Querier = brokenQuerier{run: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		return provider.FeatureQueryResult{}, nil
	}}
	if len(checkCase(instance, c)) == 0 {
		t.Fatal("dropped context cancellation accepted")
	}
}

func TestFilterCountCannotIgnorePredicate(t *testing.T) {
	c := findFilterCase(t, OrdinaryTable, "numeric equal")
	instance := filterControl(c, nil, nil, false)
	original := instance.Querier
	instance.Querier = brokenQuerier{run: func(ctx context.Context, q provider.FeatureQuery, callback func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		result, err := original.QueryFeatures(ctx, "", q, callback)
		wrong := uint64(8)
		result.NumberMatched = &wrong
		return result, err
	}}
	if len(checkCase(instance, c)) == 0 {
		t.Fatal("unfiltered count accepted")
	}
}
