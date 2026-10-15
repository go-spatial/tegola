package provider

import (
	"fmt"
	"math"
	"strconv"

	"github.com/alexeydott/geom"
)

type Feature struct {
	ID       uint64
	Geometry geom.Geometry
	SRID     uint64
	Tags     map[string]interface{}
}

// maxUint64PlusOne is the smallest float64 above the uint64 range (2^64).
// uint64's max value rounds up to exactly this value in float64, so >= is
// the correct out-of-range bound for float64 feature IDs.
const maxUint64PlusOne = 18446744073709551616.0

// ConvertFeatureID converts an interface value to a uint64 feature ID.
// Values that are not representable as a uint64 ID — negative, fractional
// or out-of-range numbers — are rejected with a clear error instead of
// being silently wrapped (-1 used to become 2^64-1) or truncated (1.5 used
// to become 1) (audit P6-10).
func ConvertFeatureID(v interface{}) (uint64, error) {
	switch aval := v.(type) {
	case float64:
		if math.IsNaN(aval) || math.IsInf(aval, 0) || aval < 0 || aval != math.Trunc(aval) || aval >= maxUint64PlusOne {
			return 0, fmt.Errorf("unable to convert feature ID %v to uint64: value is not an integer in [0, 2^64)", aval)
		}
		return uint64(aval), nil
	case int64:
		return nonNegativeFeatureID(aval, v)
	case uint64:
		return aval, nil
	case uint:
		return uint64(aval), nil
	case int8:
		return nonNegativeFeatureID(int64(aval), v)
	case uint8:
		return uint64(aval), nil
	case uint16:
		return uint64(aval), nil
	case int32:
		return nonNegativeFeatureID(int64(aval), v)
	case uint32:
		return uint64(aval), nil
	case string:
		return strconv.ParseUint(aval, 10, 64)
	case []byte:
		// MySQL drivers return TEXT/CHAR id columns as []byte
		return strconv.ParseUint(string(aval), 10, 64)
	default:
		return 0, ErrUnableToConvertFeatureID{val: v}
	}
}

// nonNegativeFeatureID converts a signed integer feature ID, rejecting
// negative values instead of wrapping them into the uint64 range.
func nonNegativeFeatureID(aval int64, original interface{}) (uint64, error) {
	if aval < 0 {
		return 0, fmt.Errorf("unable to convert feature ID %v to uint64: value is negative", original)
	}
	return uint64(aval), nil
}
