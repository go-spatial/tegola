package env

import (
	"strconv"
	"strings"

	"github.com/go-spatial/tegola/internal/p"
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
		return nil, ErrType{v}
	}
}

func ParseStringSlice(val string) ([]string, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// split
	vals := strings.Split(str, ",")

	// trim space
	for i, v := range vals {
		vals[i] = strings.TrimSpace(v)
	}

	return vals, nil
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
		return nil, ErrType{v}
	}
}

func ParseBoolSlice(val string) ([]bool, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	bools := make([]bool, 0, len(vals))
	for i := range vals {
		// trim space and parse
		b, err := strconv.ParseBool(strings.TrimSpace(vals[i]))
		if err != nil {
			return bools, err
		}

		bools = append(bools, b)
	}

	return bools, nil
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
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		i, err := strconv.Atoi(val)
		return &i, err
	default:
		return nil, ErrType{v}
	}
}

func ParseIntSlice(val string) ([]int, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	ints := make([]int, 0, len(vals))
	for i := range vals {
		// trim space and parse
		b, err := strconv.Atoi(strings.TrimSpace(vals[i]))
		if err != nil {
			return ints, err
		}

		ints = append(ints, b)
	}

	return ints, nil
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
		if val < 0 {
			return nil, ErrType{v}
		}
		return p.Uint(uint(val)), nil
	case int64:
		if val < 0 {
			return nil, ErrType{v}
		}
		return p.Uint(uint(val)), nil
	case string:
		val, err := replaceEnvVar(val)
		if err != nil {
			return nil, err
		}

		ui64, err := strconv.ParseUint(val, 10, 64)
		ui := uint(ui64)
		return &ui, err
	default:
		return nil, ErrType{v}
	}
}

func ParseUintSlice(val string) ([]uint, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	uints := make([]uint, 0, len(vals))
	for i := range vals {
		// trim space and parse
		u, err := strconv.ParseUint(strings.TrimSpace(vals[i]), 10, 64)
		if err != nil {
			return uints, err
		}

		uints = append(uints, uint(u))
	}

	return uints, nil
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
		return nil, ErrType{v}
	}
}

func ParseFloatSlice(val string) ([]float64, error) {
	// replace the env vars
	str, err := replaceEnvVar(val)
	if err != nil {
		return nil, err
	}

	// break our string up
	vals := strings.Split(str, ",")
	floats := make([]float64, 0, len(vals))
	for i := range vals {
		// trim space and parse
		f, err := strconv.ParseFloat(strings.TrimSpace(vals[i]), 64)
		if err != nil {
			return floats, err
		}

		floats = append(floats, f)
	}

	return floats, nil
}
