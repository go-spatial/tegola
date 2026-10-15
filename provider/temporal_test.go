package provider_test

import (
	"errors"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestTemporalMappingValidate(t *testing.T) {
	cases := []struct {
		name    string
		mapping provider.TemporalMapping
		invalid bool
	}{
		{name: "declared absence"},
		{name: "instant", mapping: provider.TemporalMapping{InstantField: "observed_at"}},
		{name: "interval", mapping: provider.TemporalMapping{StartField: "start_at", EndField: "end_at"}},
		{name: "literal names retained", mapping: provider.TemporalMapping{InstantField: "event time"}},
		{name: "start only", mapping: provider.TemporalMapping{StartField: "start_at"}, invalid: true},
		{name: "end only", mapping: provider.TemporalMapping{EndField: "end_at"}, invalid: true},
		{name: "instant and start", mapping: provider.TemporalMapping{InstantField: "time", StartField: "start"}, invalid: true},
		{name: "instant and end", mapping: provider.TemporalMapping{InstantField: "time", EndField: "end"}, invalid: true},
		{name: "mixed complete", mapping: provider.TemporalMapping{InstantField: "time", StartField: "start", EndField: "end"}, invalid: true},
		{name: "identical endpoints", mapping: provider.TemporalMapping{StartField: "time", EndField: "time"}, invalid: true},
		{name: "blank instant", mapping: provider.TemporalMapping{InstantField: " \t\n"}, invalid: true},
		{name: "blank start", mapping: provider.TemporalMapping{StartField: "\t", EndField: "end"}, invalid: true},
		{name: "blank end", mapping: provider.TemporalMapping{StartField: "start", EndField: "\n "}, invalid: true},
		{name: "unicode blank", mapping: provider.TemporalMapping{InstantField: "\u2003"}, invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.mapping
			err := tc.mapping.Validate()
			if tc.invalid {
				var invalid provider.InvalidFeatureQueryError
				if !errors.As(err, &invalid) || invalid.Field != "temporal" {
					t.Fatalf("want typed temporal validation error, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid mapping rejected: %v", err)
			}
			if tc.mapping != before {
				t.Fatal("validation mutated field names")
			}
		})
	}
}

type temporalMetadataStub struct {
	mapping provider.TemporalMapping
	err     error
}

func (s temporalMetadataStub) TemporalMapping() (provider.TemporalMapping, error) {
	return s.mapping, nil
}

func (s temporalMetadataStub) FeatureQuerySupported() error { return s.err }

var (
	_ provider.TemporalLayerInfo     = temporalMetadataStub{}
	_ provider.FeatureQueryLayerInfo = temporalMetadataStub{}
)

func TestTemporalMetadataValueSnapshot(t *testing.T) {
	stub := temporalMetadataStub{mapping: provider.TemporalMapping{InstantField: "time"}}
	var temporal provider.TemporalLayerInfo = stub
	mapping, err := temporal.TemporalMapping()
	if err != nil {
		t.Fatal(err)
	}
	mapping.InstantField = "changed"
	again, err := temporal.TemporalMapping()
	if err != nil || again.InstantField != "time" {
		t.Fatalf("snapshot modified metadata: %+v, %v", again, err)
	}
	var eligible provider.FeatureQueryLayerInfo = stub
	if err := eligible.FeatureQuerySupported(); err != nil {
		t.Fatal(err)
	}
	eligible = temporalMetadataStub{err: provider.ErrFeatureQueryUnsupported}
	if !errors.Is(eligible.FeatureQuerySupported(), provider.ErrUnsupported) {
		t.Fatal("unsupported eligibility lost its error chain")
	}
}
