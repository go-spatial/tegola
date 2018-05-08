package env

import (
	"strconv"
)

func ParseString(v interface{}) (*string, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}
		return &val, nil
	default:
		return nil, &ErrType{v}
	}
}

func ParseBool(v interface{}) (*bool, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case bool:
		return &val, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		b, err := strconv.ParseBool(val)
		return &b, err
	default:
		return nil, &ErrType{v}
	}
}

func ParseInt(v interface{}) (*int, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case int:
		return &val, nil
	case int64:
		i := int(val)
		return &i, nil
	case uint:
		i := int(val)
		return &i, nil
	case uint64:
		i := int(val)
		return &i, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		i, err := strconv.Atoi(val)
		return &i, err
	default:
		return nil, &ErrType{v}
	}
}

func ParseUint(v interface{}) (*uint, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case uint:
		return &val, nil
	case uint64:
		ui := uint(val)
		return &ui, nil
	case int:
		ui := uint(val)
		return &ui, nil
	case int64:
		ui := uint(val)
		return &ui, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		ui64, err := strconv.ParseUint(val, 10, 64)
		ui := uint(ui64)
		return &ui, err
	default:
		return nil, &ErrType{v}
	}
}

func ParseFloat(v interface{}) (*float64, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case float64:
		return &val, nil
	case float32:
		f := float64(val)
		return &f, nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		flt, err := strconv.ParseFloat(val, 64)
		return &flt, err
	default:
		return nil, &ErrType{v}
	}
}
