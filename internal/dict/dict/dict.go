package dict

import (
	"reflect"
	"fmt"
)

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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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

	r, ok = v.(float64)
	if !ok {
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(r)}
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
		return r, ErrKeyType{key, v,reflect.TypeOf(darr)}
	}

	r = make([]Dicter, len(darr))
	for k := range darr {
		r[k] = Dicter(Dict(darr[k]))
	}

	fmt.Printf("\n\n%v\n\n", r)

	return r, nil
}

func (d Dict) Interface(key string) (r interface{}, ok bool) {
	r, ok = d[key]
	return r, ok
}
