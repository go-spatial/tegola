package mysql

import (
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/featuresemantics"
)

func (f *featureProfile) registerTemporal(conf dict.Dicter) error {
	mapping, scale, err := featureTemporalConfig(conf)
	if err != nil {
		return err
	}
	f.temporal, f.temporalScale = mapping, scale
	for _, name := range []string{mapping.InstantField, mapping.StartField, mapping.EndField} {
		if name == "" {
			continue
		}
		source, ok := f.sourceColumn(name)
		if !ok {
			return featureInvalid("temporal", "mapped output is not projected")
		}
		column, ok := f.schema.column(source)
		if !ok {
			return featureInvalid("temporal", "mapped column missing")
		}
		bits, unsigned := integralColumn(column)
		if bits == 0 || unsigned || generatedColumn(column) {
			return featureUnsupported("temporal column must be ordinary signed integer")
		}
		canonical := name
		for _, projection := range f.projection {
			if strings.EqualFold(projection.output, name) {
				canonical = projection.output
			}
		}
		switch name {
		case mapping.InstantField:
			f.temporal.InstantField = canonical
		case mapping.StartField:
			f.temporal.StartField = canonical
		case mapping.EndField:
			f.temporal.EndField = canonical
		}
	}
	return nil
}

func featureTemporalConfig(conf dict.Dicter) (provider.TemporalMapping, int64, error) {
	keys := []string{"temporal_field", "temporal_start_field", "temporal_end_field", "temporal_storage"}
	values := make([]string, len(keys))
	for i, key := range keys {
		if raw, exists := conf.Interface(key); exists {
			value, ok := raw.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return provider.TemporalMapping{}, 0, featureInvalid(key, "must be a nonblank string")
			}
			values[i] = value
		}
	}
	mapping := provider.TemporalMapping{InstantField: values[0], StartField: values[1], EndField: values[2]}
	if err := mapping.Validate(); err != nil {
		return mapping, 0, err
	}
	if mapping == (provider.TemporalMapping{}) {
		if values[3] != "" {
			return mapping, 0, featureInvalid("temporal_storage", "requires temporal fields")
		}
		return mapping, 0, nil
	}
	var scale int64
	switch values[3] {
	case "unix_seconds":
		scale = 1
	case "unix_milliseconds":
		scale = 1000
	case "unix_microseconds":
		scale = 1000000
	case "unix_nanoseconds":
		scale = 1000000000
	default:
		return mapping, 0, featureInvalid("temporal_storage", "requires an integer POSIX storage profile")
	}
	return mapping, scale, nil
}

func (f *featureProfile) sourceColumn(output string) (string, bool) {
	for _, projection := range f.projection {
		if strings.EqualFold(projection.output, output) {
			return projection.source, true
		}
	}
	return "", false
}

func exactFeatureEpochBound(t time.Time, scale int64, ceil bool, fraction string, leap bool) *big.Int {
	return featuresemantics.EpochBound(t, scale, ceil, fraction, leap)
}

func featureSignedInteger(value any) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	var number int64
	var err error
	switch v := value.(type) {
	case int64:
		number = v
	case []byte:
		number, err = strconv.ParseInt(string(v), 10, 64)
	default:
		return nil, featureInvalid("temporal", "source is not an integer")
	}
	if err != nil {
		return nil, featureInvalid("temporal", "source integer out of range")
	}
	return &number, nil
}

func (f *featureProfile) temporalMatches(values map[string]any, temporal *provider.TemporalConstraint) (bool, error) {
	m := f.temporal
	if m == (provider.TemporalMapping{}) {
		return true, nil
	}
	start, err := featureSignedInteger(values[m.StartField])
	if err != nil {
		return false, err
	}
	end, err := featureSignedInteger(values[m.EndField])
	if err != nil {
		return false, err
	}
	if m.InstantField != "" {
		start, err = featureSignedInteger(values[m.InstantField])
		if err != nil {
			return false, err
		}
		end = start
	}
	if start != nil && end != nil && *start > *end {
		return false, featureInvalid("temporal", "source interval reversed")
	}
	if temporal == nil || (start == nil && end == nil) {
		return true, nil
	}
	if temporal.Start != nil && end != nil {
		bound := exactFeatureEpochBound(*temporal.Start, f.temporalScale, true, temporal.StartSubNanosecond, temporal.StartLeapSecond)
		if big.NewInt(*end).Cmp(bound) < 0 {
			return false, nil
		}
	}
	if temporal.End != nil && start != nil {
		bound := exactFeatureEpochBound(*temporal.End, f.temporalScale, false, temporal.EndSubNanosecond, temporal.EndLeapSecond)
		if big.NewInt(*start).Cmp(bound) > 0 {
			return false, nil
		}
	}
	return true, nil
}
