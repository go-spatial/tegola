package querytest

import (
	"context"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func nullableLiteralDelivery(c caseSpec) []provider.Feature {
	features := staticDomainDelivery(c, c.ids)
	for i := range features {
		if reflect.DeepEqual(c.query.Fields, []string{"name"}) {
			features[i].Tags = map[string]any{"name": features[i].Tags["name"]}
		}
		if reflect.DeepEqual(c.query.Fields, []string{"value"}) {
			features[i].Tags = map[string]any{"value": features[i].Tags["value"]}
		}
	}
	return features
}

func nullableControl(c caseSpec, shared bool) Instance {
	features := nullableLiteralDelivery(c)
	return Instance{CountMode: ExactCount, Querier: brokenQuerier{run: func(
		_ context.Context, _ provider.FeatureQuery, callback func(*provider.Feature) error,
	) (provider.FeatureQueryResult, error) {
		for i := range features {
			feature := cloneFeature(features[i])
			if shared {
				if err := callback(&features[i]); err != nil {
					return provider.FeatureQueryResult{}, err
				}
			} else if err := callback(&feature); err != nil {
				return provider.FeatureQueryResult{}, err
			}
		}
		count := c.matched
		return provider.FeatureQueryResult{NumberReturned: uint64(len(features)), NumberMatched: &count, HasMore: c.more}, nil
	}}}
}

func TestNullableLiteralPublicationAndBadControls(t *testing.T) {
	for _, profile := range []FixtureProfile{OrdinaryTable, CustomSelection} {
		for _, c := range nullablePropertyCases(profile) {
			t.Run(c.name+unitProfileName(profile), func(t *testing.T) {
				if issues := checkCase(nullableControl(c, false), c); len(issues) != 0 {
					t.Fatal(issues)
				}
				count := c.matched
				result := provider.FeatureQueryResult{NumberReturned: uint64(len(c.ids)), NumberMatched: &count, HasMore: c.more}
				for _, kind := range []string{"drop nil", "zero/empty corruption", "unselected nil injection"} {
					bad := nullableLiteralDelivery(c)
					changed := false
					for i := range bad {
						for key, value := range bad[i].Tags {
							if kind == "drop nil" && value == nil {
								delete(bad[i].Tags, key)
								changed = true
							}
							if kind == "zero/empty corruption" && (value == int64(0) || value == "") {
								bad[i].Tags[key] = nil
								changed = true
							}
						}
						if kind == "unselected nil injection" && len(c.query.Fields) == 1 {
							if c.query.Fields[0] == "name" {
								bad[i].Tags["value"] = nil
							} else {
								bad[i].Tags["name"] = nil
							}
							changed = true
						}
					}
					if changed && len(checkResponse(result, bad, c, ExactCount)) == 0 {
						t.Fatalf("control escaped: %s", kind)
					}
				}
				if profile == CustomSelection {
					bad := staticDomainDelivery(c, []uint64{90})
					wrongCount := uint64(1)
					if len(checkResponse(provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &wrongCount}, bad, c, ExactCount)) == 0 {
						t.Fatal("excluded custom source delivered")
					}
				}
				if c.mutateDelivery {
					if len(checkCase(nullableControl(c, true), c)) == 0 {
						t.Fatal("shared nullable map accepted")
					}
					if issues := checkDimensionalConcurrent(nullableControl(c, false), c); len(issues) != 0 {
						t.Fatal(issues)
					}
				}
			})
		}
	}
}

func unitProfileName(profile FixtureProfile) string {
	if profile == CustomSelection {
		return " custom"
	}
	return " ordinary"
}

func TestFixturePublicDeclarationCompletenessAndOwnership(t *testing.T) {
	for _, profile := range []FixtureProfile{OrdinaryTable, CustomSelection} {
		original := nullablePropertyFixture(profile)
		before := cloneFixture(original)
		declared, err := declareFixturePublicFields(original)
		if err != nil || !reflect.DeepEqual(declared.PublicFields, []string{"name", "value"}) {
			t.Fatal(declared.PublicFields, err)
		}
		declared.PublicFields[0] = "corrupt"
		declared.Rows[0].Feature.Tags["name"] = "corrupt"
		if !reflect.DeepEqual(original, before) {
			t.Fatal("declaration mutated literal oracle")
		}
		delete(original.Rows[0].Feature.Tags, "value")
		if _, err := declareFixturePublicFields(original); err == nil {
			t.Fatal("mixed sparse rows accepted/fabricated nil")
		}
		if _, ok := original.Rows[0].Feature.Tags["value"]; ok {
			t.Fatal("invalid oracle map rewritten")
		}
	}
	for _, exception := range []Row{
		{Metadata: true}, {MissingID: true}, {MalformedGeometry: "wkb"},
	} {
		fixture := nullablePropertyFixture(OrdinaryTable)
		fixture.Rows = append(fixture.Rows, exception)
		if _, err := declareFixturePublicFields(fixture); err != nil {
			t.Fatal("documented nonfeature exception", err)
		}
	}
	excluded := nullablePropertyFixture(CustomSelection)
	delete(excluded.Rows[3].Feature.Tags, "name")
	if _, err := declareFixturePublicFields(excluded); err == nil {
		t.Fatal("excluded representable row escaped completeness")
	}
	for _, fields := range [][]string{nil, {}, {"name", "value"}} {
		fixture := Fixture{PublicFields: fields}
		copy := cloneFixture(fixture)
		if (copy.PublicFields == nil) != (fields == nil) {
			t.Fatal("nil/explicit-none declaration conflated")
		}
		if len(copy.PublicFields) > 0 {
			copy.PublicFields[0] = "mutated"
			if fields[0] == "mutated" {
				t.Fatal("public fields backing shared")
			}
		}
	}
}

func TestAllExistingLiteralFixturesHaveConsistentPublicProjection(t *testing.T) {
	fixtures := []Fixture{canonicalFixture(), customDomainFixture()}
	for _, c := range contractCases() {
		fixtures = append(fixtures, c.fixture)
	}
	for _, c := range dimensionalCases() {
		fixtures = append(fixtures, c.fixture)
	}
	for _, unit := range []TemporalPropertyStorage{UnixSeconds, UnixMilliseconds, UnixMicroseconds, UnixNanoseconds} {
		for _, c := range exactTemporalCases(unit) {
			fixtures = append(fixtures, c.fixture)
		}
	}
	for _, fixture := range fixtures {
		declared, err := declareFixturePublicFields(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if declared.PublicFields == nil {
			t.Fatal("suite declaration left unspecified")
		}
		storage := fixture.TemporalStorage
		if storage == 0 {
			storage = UnixNanoseconds
		}
		profiled, err := withPublicTemporalProperties(fixture, TemporalPropertyProfile{
			StartField: "start_time", EndField: "end_time", Storage: storage,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := declareFixturePublicFields(profiled); err != nil {
			t.Fatal("public temporal completeness", err)
		}
	}
}
