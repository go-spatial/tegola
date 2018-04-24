package provider

import (
	"strconv"
	"time"

	"github.com/go-spatial/geom"
)

type TimePeriod [2]*time.Time

func (tp *TimePeriod) StartTime() *time.Time {
	return tp[0]
}

func (tp *TimePeriod) EndTime() *time.Time {
	return tp[1]
}

type Feature struct {
	ID         uint64
	Geometry   geom.Geometry
	SRID       uint64
	Properties map[string]interface{} // Renamed Tags -> Properties

	// Time values for features with time data.  A timestamp would be represented by identical
	//	values.  For features w/o time data, nil value for both start & stop.
	Time TimePeriod

	// This can be generated in a number of ways, for example; hashing the underlying data,
	//	from a modification timestamp, or from a version number.  For read-only data,
	//	a concatenation of the table name & string representation of the feature's ID.
	//	If the provider doesn't support or can't provide this value, it can set this to nil.
	// It needs to be reasonably unique across features in the provider (like the probability of a hash collision)
	// It needs to remain the same if the underlying data doesn't change
	// It needs to change if the underying data changes
	ModificationTag *string
}

// ConvertFeatureID attempts to convert an interface value to an uint64
func ConvertFeatureID(v interface{}) (uint64, error) {
	switch aval := v.(type) {
	case float64:
		return uint64(aval), nil
	case int64:
		return uint64(aval), nil
	case uint64:
		return aval, nil
	case uint:
		return uint64(aval), nil
	case int8:
		return uint64(aval), nil
	case uint8:
		return uint64(aval), nil
	case uint16:
		return uint64(aval), nil
	case int32:
		return uint64(aval), nil
	case uint32:
		return uint64(aval), nil
	case string:
		return strconv.ParseUint(aval, 10, 64)
	default:
		return 0, ErrUnableToConvertFeatureID{val: v}
	}
}
