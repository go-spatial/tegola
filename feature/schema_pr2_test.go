package feature

import (
	"context"
	"github.com/alexeydott/tegola/provider"
	"testing"
)

func TestPR2ExplicitNullDoesNotUseDefault(t *testing.T) {
	schema := &SchemaDescriptor{Properties: []PropertyDescriptor{{Name: "count", Type: TypeInteger, HasDefault: true}}}
	if err := schema.ValidateInputValue("count", TypedValue{State: ValueNull}); err == nil {
		t.Fatal("NOT NULL DEFAULT accepted explicit null")
	}
	if err := schema.ValidateInputValue("count", TypedValue{State: ValueAbsent}); err != nil {
		t.Fatal(err)
	}
}

func TestPR2DateTimeRequiresRFC3339(t *testing.T) {
	schema := &SchemaDescriptor{Properties: []PropertyDescriptor{{Name: "instant", Type: TypeDateTime}}}
	for _, value := range []string{"not-a-date", "2026-01-01T12:00:00,5Z", "2026-01-01T1:00:00Z", "2026-01-01T12:00:00+24:00", "2026-02-30T12:00:00Z", "2026-10-04", "2026-10-04T12:00:00", ""} {
		if err := schema.ValidateInputValue("instant", TypedValue{Type: TypeDateTime, State: ValuePresent, String: value}); err == nil {
			t.Errorf("accepted invalid timestamp %q", value)
		}
	}
	for _, value := range []string{"2026-10-04T12:00:00Z", "2026-10-04T12:00:00.123456789+03:00"} {
		if err := schema.ValidateInputValue("instant", TypedValue{Type: TypeDateTime, State: ValuePresent, String: value}); err != nil {
			t.Errorf("rejected timestamp %q: %v", value, err)
		}
	}
}

func TestPR2GeometryNullability(t *testing.T) {
	for _, nullable := range []bool{false, true} {
		schema := &SchemaDescriptor{Geometry: GeometryDescriptor{Nullable: nullable}}
		err := validateMutationInput(schema, provider.Mutation{Op: provider.MutationUpdate, FeatureID: 1, GeometryAbsent: true})
		if (err == nil) != nullable {
			t.Errorf("nullable=%v clear error=%v", nullable, err)
		}
		if err := validateMutationInput(schema, provider.Mutation{Op: provider.MutationUpdate, FeatureID: 1}); err != nil {
			t.Fatalf("absent geometry is not a clear: %v", err)
		}
	}
}

func TestPR2InvalidValuesRejectBeforeProviderResolution(t *testing.T) {
	for _, op := range []provider.MutationOp{provider.MutationInsert, provider.MutationReplace, provider.MutationUpdate} {
		t.Run(op.String(), func(t *testing.T) {
			schema := &SchemaDescriptor{Geometry: GeometryDescriptor{Nullable: false}, Properties: []PropertyDescriptor{{Name: "instant", Type: TypeDateTime}}}
			coordinator := MutationCoordinator{
				PolicyFor: func(string) Policy { return AllowAllPolicy{} },
				SchemaFor: func(string) (*SchemaDescriptor, error) { return schema, nil },
				ProviderFor: func(string) (provider.MutationProvider, string, error) {
					t.Fatal("invalid input reached provider resolution")
					return nil, "", nil
				},
			}
			id := uint64(1)
			if op == provider.MutationInsert {
				id = 0
			}
			invalid := []provider.Mutation{
				{Op: op, Collection: "sites", FeatureID: id, GeometryAbsent: true},
				{Op: op, Collection: "sites", FeatureID: id, Properties: map[string]provider.MutationValue{"instant": {Kind: provider.MutationValueString, String: "not-a-date"}}},
			}
			if op != provider.MutationUpdate {
				invalid = append(invalid, provider.Mutation{Op: op, Collection: "sites", FeatureID: id})
			}
			for _, m := range invalid {
				_, _, err := coordinator.Execute(context.Background(), Principal{}, m)
				if classified, ok := provider.AsMutationError(err); !ok || classified.Kind != provider.MutationErrSchemaViolation {
					t.Fatalf("expected schema violation: %v", err)
				}
			}
		})
	}
}

func TestPR3DateTimeAcceptsOnlyAnnouncedLeapSeconds(t *testing.T) {
	schema := &SchemaDescriptor{Properties: []PropertyDescriptor{{Name: "instant", Type: TypeDateTime}}}
	for _, value := range []string{
		"2016-12-31T23:59:60Z", "2016-12-31T23:59:60.123456789012Z",
		"2017-01-01T02:59:60+03:00", "2016-12-31T18:59:60-05:00",
		"2016-12-31t23:59:60z", "2026-10-04t12:00:00z",
	} {
		if err := schema.ValidateInputValue("instant", TypedValue{Type: TypeDateTime, State: ValuePresent, String: value}); err != nil {
			t.Errorf("valid RFC3339 rejected %q: %v", value, err)
		}
	}
	for _, value := range []string{
		"2017-12-31T23:59:60Z", "2016-12-31T22:59:60Z", "2017-01-01T03:59:60+03:00",
		"2016-12-31T23:59:61Z", "2016-12-31T23:59:60,5Z", "2016-12-31T23:59:60+24:00",
	} {
		if err := schema.ValidateInputValue("instant", TypedValue{Type: TypeDateTime, State: ValuePresent, String: value}); err == nil {
			t.Errorf("invalid RFC3339 accepted %q", value)
		}
	}
}
