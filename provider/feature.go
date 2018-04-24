package provider

import (
	"strconv"

	"github.com/go-spatial/geom"
)

type Feature struct {
	ID       uint64
	Geometry geom.Geometry
	SRID     uint64
	Tags     map[string]interface{}
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
