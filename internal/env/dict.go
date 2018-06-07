/* This file was generated using gen.pl and go fmt. */

// dict is a helper function that allow one to easily get concreate values out of a map[string]interface{}
package env

import (
	"fmt"
	"reflect"

	"github.com/go-spatial/tegola/internal/dict"
)

type Dict map[string]interface{}

// Dict is to obtain a map[string]interface{} that has already been cast to a M type.
func (d Dict) Dict(key string) (v Dict, err error) {
	var val interface{}
	var dv Dict
	var ok bool
	if val, ok = d[key]; !ok {
		return v, fmt.Errorf("%v value is required.", key)
	}

	if dv, ok = val.(Dict); !ok {
		return v, fmt.Errorf("%v value needs to be of type map[string]interface{}.", key)
	}
	return dv, nil
}

// String returns the value as a string type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (d Dict) String(key string, def *string) (v string, err error) {
	var val interface{}
	var ok bool

	if val, ok = d[key]; !ok || val == nil {
		if def != nil {
			return *def, nil
		}
		return v, dict.ErrKeyRequired(key)
	}

	ptr, err := ParseString(val)
	if err != nil {
		switch err.(type) {
		case ErrEnvVar:
			return v, err
		default:
			return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
		}
	}
	return *ptr, nil
}

func (d Dict) StringSlice(key string) (v []string, err error) {

	val, ok := d[key]
	if !ok {
		return v, nil
	}

	switch val.(type) {
	case string:
		v, err = ParseStringSlice(val.(string))
		if err != nil {
			switch err.(type) {
			case ErrEnvVar:
				return v, err
			default:
				return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
			}
		}

	case []string:
		v = val.([]string)
	case []interface{}:
		// It's possible that the value is of type []interface and not of our type, so we need to convert each element to the appropriate
		// type first, and then into the this type.
		var iv []interface{}
		if iv, ok = val.([]interface{}); !ok {
			// Could not convert to the generic type, so we don't have the correct thing.
			return v, dict.ErrKeyType{key, val, reflect.TypeOf(iv)}
		}

		v = make([]string, len(iv))
		for k := range iv {
			if iv[k] == nil {
				v[k] = ""
			} else {
				ptr, err := ParseString(val)
				if err != nil {
					switch err.(type) {
					case ErrEnvVar:
						return v, err
					default:
						return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
					}
				}

				v[k] = *ptr
			}
		}
	}

	return v, nil
}

// Bool returns the value as a string type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (d Dict) Bool(key string, def *bool) (v bool, err error) {
	var val interface{}
	var ok bool

	if val, ok = d[key]; !ok || val == nil {
		if def != nil {
			return *def, nil
		}
		return v, dict.ErrKeyRequired(key)
	}

	ptr, err := ParseBool(val)
	if err != nil {
		switch err.(type) {
		case ErrEnvVar:
			return v, err
		default:
			return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
		}
	}

	return *ptr, nil
}

func (d Dict) BoolSlice(key string) (v []bool, err error) {

	val, ok := d[key]
	if !ok {
		return v, nil
	}

	switch val.(type) {
	case string:
		v, err = ParseBoolSlice(val.(string))
		if err != nil {
			switch err.(type) {
			case ErrEnvVar:
				return v, err
			default:
				return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
			}
		}
	case []bool:
		v = val.([]bool)
	case []interface{}:
		// It's possible that the value is of type []interface and not of our type, so we need to convert each element to the appropriate
		// type first, and then into the this type.
		var iv []interface{}
		if iv, ok = val.([]interface{}); !ok {
			// Could not convert to the generic type, so we don't have the correct thing.
			return v, dict.ErrKeyType{key, val, reflect.TypeOf(iv)}
		}

		v = make([]bool, len(iv))
		for k := range iv {
			if iv[k] == nil {
				iv[k] = false
			} else {
				ptr, err := ParseBool(val)
				if err != nil {
					switch err.(type) {
					case ErrEnvVar:
						return v, err
					default:
						return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
					}
				}

				v[k] = *ptr
			}
		}
	}

	return v, nil
}

// Int returns the value as a int type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (d Dict) Int(key string, def *int) (v int, err error) {
	var val interface{}
	var ok bool
	if val, ok = d[key]; !ok || val == nil {
		if def != nil {
			return *def, nil
		}
		return v, dict.ErrKeyRequired(key)
	}

	ptr, err := ParseInt(val)
	if err != nil {
		switch err.(type) {
		case ErrEnvVar:
			return v, err
		default:
			return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
		}
	}

	return *ptr, nil
}

func (d Dict) IntSlice(key string) (v []int, err error) {

	val, ok := d[key]
	if !ok {
		return v, nil
	}

	switch val.(type) {
	case string:
		v, err = ParseIntSlice(val.(string))
		if err != nil {
			switch err.(type) {
			case ErrEnvVar:
				return v, err
			default:
				return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
			}
		}
	case []int:
		v = val.([]int)
	case []interface{}:
		// It's possible that the value is of type []interface and not of our type, so we need to convert each element to the appropriate
		// type first, and then into the this type.
		var iv []interface{}
		if iv, ok = val.([]interface{}); !ok {
			// Could not convert to the generic type, so we don't have the correct thing.
			return v, dict.ErrKeyType{key, val, reflect.TypeOf(iv)}
		}
		v = make([]int, len(iv))
		for k := range iv {
			if iv[k] == nil {
				iv[k] = 0
			} else {
				ptr, err := ParseInt(val)
				if err != nil {
					switch err.(type) {
					case ErrEnvVar:
						return v, err
					default:
						return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
					}
				}

				v[k] = *ptr
			}

		}
	}

	return v, nil
}

// Uint returns the value as a uint type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (d Dict) Uint(key string, def *uint) (v uint, err error) {
	var val interface{}
	var ok bool
	if val, ok = d[key]; !ok || val == nil {
		if def != nil {
			return *def, nil
		}
		return v, dict.ErrKeyRequired(key)
	}

	ptr, err := ParseUint(val)
	if err != nil {
		switch err.(type) {
		case ErrEnvVar:
			return v, err
		default:
			return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
		}
	}

	return *ptr, nil
}

func (d Dict) UintSlice(key string) (v []uint, err error) {

	val, ok := d[key]
	if !ok {
		return v, nil
	}

	switch val.(type) {
	case string:
		v, err = ParseUintSlice(val.(string))
		if err != nil {
			switch err.(type) {
			case ErrEnvVar:
				return v, err
			default:
				return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
			}
		}
	case []uint:
		v = val.([]uint)
	case []interface{}:
		// It's possible that the value is of type []interface and not of our type, so we need to convert each element to the appropriate
		// type first, and then into the this type.
		var iv []interface{}
		if iv, ok = val.([]interface{}); !ok {
			// Could not convert to the generic type, so we don't have the correct thing.
			return v, &ErrType{val}
		}
		for k := range iv {
			if iv[k] == nil {
				iv[k] = 0
			} else {
				ptr, err := ParseUint(val)
				if err != nil {
					switch err.(type) {
					case ErrEnvVar:
						return v, err
					default:
						return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
					}
				}

				v[k] = *ptr
			}
		}
	}

	return v, nil
}

// Float returns the value as a uint type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (d Dict) Float(key string, def *float64) (v float64, err error) {
	var val interface{}
	var ok bool
	if val, ok = d[key]; !ok || val == nil {
		if def != nil {
			return *def, nil
		}
		return v, dict.ErrKeyRequired(key)
	}

	ptr, err := ParseFloat(val)
	if err != nil {
		switch err.(type) {
		case ErrEnvVar:
			return v, err
		default:
			return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
		}
	}

	return *ptr, nil
}

func (d Dict) FloatSlice(key string) (v []float64, err error) {
	val, ok := d[key]
	if !ok {
		return v, nil
	}

	switch val.(type) {
	case string:
		v, err = ParseFloatSlice(val.(string))
		if err != nil {
			switch err.(type) {
			case ErrEnvVar:
				return v, err
			default:
				return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
			}
		}
	case []float64:
		v = val.([]float64)
	case []interface{}:
		// It's possible that the value is of type []interface and not of our type, so we need to convert each element to the appropriate
		// type first, and then into the this type.
		var iv []interface{}
		if iv, ok = val.([]interface{}); !ok {
			// Could not convert to the generic type, so we don't have the correct thing.
			return v, &ErrType{val}
		}
		for k := range iv {
			if iv[k] == nil {
				iv[k] = 0
			} else {
				ptr, err := ParseFloat(val)
				if err != nil {
					switch err.(type) {
					case ErrEnvVar:
						return v, err
					default:
						return v, dict.ErrKeyType{Key: key, Value: val, T: reflect.TypeOf(v)}
					}
				}

				v[k] = *ptr
			}
		}
	}

	return v, nil
}

func (d Dict) Map(key string) (r dict.Dicter, err error) {
	v, ok := d[key]
	if !ok {
		// TODO(@ear7h): Revise this behavior, replicated from util/dict.Map
		return Dict{}, nil
	}

	r, ok = v.(Dict)
	if !ok {
		switch err.(type) {
		case ErrEnvVar:
			return r, err
		default:
			return r, dict.ErrKeyType{Key: key, Value: r, T: reflect.TypeOf(v)}
		}
	}

	return r, nil
}

func (d Dict) MapSlice(key string) (r []dict.Dicter, err error) {
	v, ok := d[key]
	if !ok {
		// TODO(@ear7h): Revise this behavior, replicated from util/dict.Map
		return r, nil
	}

	arr, ok := v.([]map[string]interface{})
	if !ok {
		return r, dict.ErrKeyType{key, v, reflect.TypeOf(arr)}
	}

	r = make([]dict.Dicter, len(arr))
	for k := range arr {
		r[k] = dict.Dicter(Dict(arr[k]))
	}

	return r, nil
}

func (d Dict) Interface(key string) (v interface{}, ok bool) {
	v, ok = d[key]
	return v, ok
}
