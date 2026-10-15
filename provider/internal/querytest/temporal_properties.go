package querytest

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

type TemporalPropertyStorage uint8

const (
	UnixSeconds TemporalPropertyStorage = iota + 1
	UnixMilliseconds
	UnixMicroseconds
	UnixNanoseconds
)

// TemporalPropertyProfile describes explicitly projected public time columns.
// Both aliases are present; an absent source value is a nil property.
type TemporalPropertyProfile struct {
	StartField, EndField string
	Storage              TemporalPropertyStorage
}

type Options struct {
	NativeProfileOutcomes *NativeProfileOptions
	// NativeRingOrientationEquivalent is test-only for trusted native-export profiles.
	// Raw WKB/WKT profiles must leave it false. Only exact fixed-start ring reversal is equivalent.
	NativeRingOrientationEquivalent bool
	PublicTemporalProperties        *TemporalPropertyProfile
}

// EncodeFixtureTemporal encodes a literal source instant exactly. It is not a
// query endpoint quantizer and rejects nonrepresentability instead of rounding.
func EncodeFixtureTemporal(value time.Time, storage TemporalPropertyStorage) (int64, error) {
	encoded, err := fixtureTemporalInteger(&value, storage)
	if err != nil {
		return 0, err
	}
	return encoded.(int64), nil
}

func copyTemporalPropertyProfile(profile *TemporalPropertyProfile) (*TemporalPropertyProfile, error) {
	if profile == nil {
		return nil, nil
	}
	copy := *profile
	if strings.TrimSpace(copy.StartField) == "" || strings.TrimSpace(copy.EndField) == "" ||
		copy.StartField == copy.EndField || copy.Storage < UnixSeconds || copy.Storage > UnixNanoseconds {
		return nil, fmt.Errorf("querytest: invalid public temporal property profile")
	}
	return &copy, nil
}

func withPublicTemporalProperties(fixture Fixture, profile TemporalPropertyProfile) (Fixture, error) {
	copy := cloneFixture(fixture)
	for i := range copy.Rows {
		row := &copy.Rows[i]
		if _, exists := row.Feature.Tags[profile.StartField]; exists {
			return Fixture{}, fmt.Errorf("querytest: temporal alias overlaps fixture properties")
		}
		if _, exists := row.Feature.Tags[profile.EndField]; exists {
			return Fixture{}, fmt.Errorf("querytest: temporal alias overlaps fixture properties")
		}
		if row.Feature.Tags == nil {
			row.Feature.Tags = map[string]any{}
		}
		start, err := fixtureTemporalInteger(row.Start, profile.Storage)
		if err != nil {
			return Fixture{}, err
		}
		end, err := fixtureTemporalInteger(row.End, profile.Storage)
		if err != nil {
			return Fixture{}, err
		}
		row.Feature.Tags[profile.StartField], row.Feature.Tags[profile.EndField] = start, end
		if row.RawTemporal != nil {
			if fixture.TemporalStorage == 0 || fixture.TemporalStorage != profile.Storage {
				return Fixture{}, fmt.Errorf("querytest: raw temporal ticks require matching declared storage")
			}
			var start, end any
			if row.RawTemporal.Start != nil {
				start = *row.RawTemporal.Start
			}
			if row.RawTemporal.End != nil {
				end = *row.RawTemporal.End
			}
			row.Feature.Tags[profile.StartField], row.Feature.Tags[profile.EndField] = start, end
		}
	}
	return copy, nil
}

// This is an integer representation of declared fixture source times, never a
// production query-bound predicate. It rejects loss rather than flooring.
func fixtureTemporalInteger(value *time.Time, storage TemporalPropertyStorage) (any, error) {
	if value == nil {
		return nil, nil
	}
	var factor int64
	switch storage {
	case UnixSeconds:
		factor = 1
	case UnixMilliseconds:
		factor = 1000
	case UnixMicroseconds:
		factor = 1000000
	case UnixNanoseconds:
		factor = 1000000000
	default:
		return nil, fmt.Errorf("querytest: unknown temporal property storage")
	}
	divisor := int64(1000000000) / factor
	if int64(value.Nanosecond())%divisor != 0 {
		return nil, fmt.Errorf("querytest: fixture time is not exactly representable in declared storage")
	}
	result := new(big.Int).Mul(big.NewInt(value.Unix()), big.NewInt(factor))
	result.Add(result, big.NewInt(int64(value.Nanosecond())/divisor))
	if !result.IsInt64() {
		return nil, fmt.Errorf("querytest: fixture temporal integer overflows storage")
	}
	return result.Int64(), nil
}
