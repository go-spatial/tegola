package dict

import "reflect"

// Dict is a pass-through implementation of the Dicter interface
type Dict map[string]interface{}

func (d Dict) String(key string, def *string) (r string, err error) {
	v, ok := d[key]
	if !ok {
		if def == nil {
			return r, ErrKeyRequired(key)
		} else {
			return *def, nil
		}
	}

	r, ok = v.(string)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) StringSlice(key string) (r []string, err error) {
	v, ok := d[key]
	if !ok {
		return r, nil
	}

	r, ok = v.([]string)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) Bool(key string, def *bool) (r bool, err error) {
	v, ok := d[key]
	if !ok {
		if def == nil {
			return r, nil
		} else {
			return *def, nil
		}
	}

	r, ok = v.(bool)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) BoolSlice(key string) (r []bool, err error) {
	v, ok := d[key]
	if !ok {
		return r, nil
	}

	r, ok = v.([]bool)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) Int(key string, def *int) (r int, err error) {
	v, ok := d[key]
	if !ok {
		if def == nil {
			return r, ErrKeyRequired(key)
		} else {
			return *def, nil
		}
	}

	r, ok = v.(int)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) IntSlice(key string) (r []int, err error) {
	v, ok := d[key]
	if !ok {
		return r, nil
	}

	r, ok = v.([]int)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) Uint(key string, def *uint) (r uint, err error) {
	v, ok := d[key]
	if !ok {
		if def == nil {
			return r, nil
		} else {
			return *def, nil
		}
	}

	r, ok = v.(uint)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) UintSlice(key string) (r []uint, err error) {
	v, ok := d[key]
	if !ok {
		return r, nil
	}

	r, ok = v.([]uint)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) Float(key string, def *float64) (r float64, err error) {
	v, ok := d[key]
	if !ok {
		if def == nil {
			return r, ErrKeyRequired(key)
		} else {
			return *def, nil
		}
	}

	// accept the whole Go integer family: TOML maps bare literals like
	// mos_precision = 2 to int64/int, not float64
	switch n := v.(type) {
	case float64:
		r = n
	case float32:
		r = float64(n)
	case int:
		r = float64(n)
	case int8:
		r = float64(n)
	case int16:
		r = float64(n)
	case int32:
		r = float64(n)
	case int64:
		r = float64(n)
	case uint:
		r = float64(n)
	case uint8:
		r = float64(n)
	case uint16:
		r = float64(n)
	case uint32:
		r = float64(n)
	case uint64:
		r = float64(n)
	case uintptr:
		r = float64(n)
	default:
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) FloatSlice(key string) (r []float64, err error) {
	v, ok := d[key]
	if !ok {
		return r, nil
	}

	r, ok = v.([]float64)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) Map(key string) (r Dicter, err error) {
	v, ok := d[key]
	if !ok {
		return Dict{}, nil
	}

	r, ok = v.(Dict)
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(r)}
	}

	return r, nil
}

func (d Dict) MapSlice(key string) (r []Dicter, err error) {
	v, ok := d[key]
	if !ok {
		return r, nil
	}

	darr, ok := v.([]map[string]interface{})
	if !ok {
		return r, ErrKeyType{Key: key, Value: v, T: reflect.TypeOf(darr)}
	}

	r = make([]Dicter, len(darr))
	for k := range darr {
		r[k] = Dicter(Dict(darr[k]))
	}

	return r, nil
}

func (d Dict) Interface(key string) (r interface{}, ok bool) {
	r, ok = d[key]
	return r, ok
}
