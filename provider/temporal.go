package provider

import (
	"strings"

	"github.com/alexeydott/tegola/internal/log"
)

// TemporalMapping identifies immutable source fields describing temporal geometry.
// Its zero value declares absence. A mapping names either one instant field or
// both interval endpoint fields. Source existence, alias identity, data types and
// storage normalization are validated by the provider during registration.
type TemporalMapping struct {
	InstantField string
	StartField   string
	EndField     string
}

// Validate checks mapping shape without changing field names or source metadata.
func (m TemporalMapping) Validate() error {
	log.Logger().Debug("validating provider temporal mapping")
	defer log.Logger().Debug("provider temporal mapping validation finished")
	fields := []string{m.InstantField, m.StartField, m.EndField}
	for _, field := range fields {
		if field != "" && strings.TrimSpace(field) == "" {
			return InvalidFeatureQueryError{Field: "temporal", Reason: "contains a blank source field name"}
		}
	}
	hasInstant := m.InstantField != ""
	hasInterval := m.StartField != "" || m.EndField != ""
	if hasInstant && hasInterval {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "instant and interval mappings cannot be combined"}
	}
	if hasInterval && (m.StartField == "" || m.EndField == "") {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "interval requires both source field names"}
	}
	if hasInterval && m.StartField == m.EndField {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "interval source field names must differ"}
	}
	return nil
}

// TemporalLayerInfo is an optional layer capability providing a value snapshot
// of registration-finalized temporal metadata. A zero mapping explicitly declares
// absence of temporal geometry; lacking this capability means unknown metadata.
// Implementations expose no setters and must not mutate metadata during queries.
type TemporalLayerInfo interface {
	TemporalMapping() (TemporalMapping, error)
}

// FeatureQueryLayerInfo declares registration-validated raw-feature eligibility.
// A nil error declares support; unsupported profiles return an error matching
// ErrUnsupported through errors.Is. Lacking the capability does not declare
// support. These optional capabilities leave tile-only LayerInfo users unchanged.
type FeatureQueryLayerInfo interface {
	FeatureQuerySupported() error
}
