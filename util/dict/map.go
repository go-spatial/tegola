/* This file was generated using gen.pl and go fmt. */
// dict is a helper function that allow one to easily get concreate values out of a map[string]interface{}
package dict

import "fmt"

type M map[string]interface{}

// Dict is to obtain a map[string]interface{} that has already been cast to a M type.
func (m M) Dict(key string) (v M, err error) {
	var val interface{}
	var mv map[string]interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, fmt.Errorf("%v value is required.", key)
	}

	if mv, ok = val.(map[string]interface{}); !ok {
		return v, fmt.Errorf("%v value needs to be of type map[string]interface{}.", key)
	}
	return M(mv), nil
}

// String returns the value as a string type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) String(key string, def *string) (v string, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(string); !ok {
		return *def, fmt.Errorf("%v value needs to be of type string.", key)
	}
	return v, nil
}

func (m M) StringSlice(key string) (v []string, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]string); !ok {
		return v, fmt.Errorf("%v value needs to be of type []string.", key)
	}
	return v, nil
}

// Int returns the value as a int type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Int(key string, def *int) (v int, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(int); !ok {
		return *def, fmt.Errorf("%v value needs to be of type int.", key)
	}
	return v, nil
}

func (m M) IntSlice(key string) (v []int, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]int); !ok {
		return v, fmt.Errorf("%v value needs to be of type []int.", key)
	}
	return v, nil
}

// Uint returns the value as a uint type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Uint(key string, def *uint) (v uint, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(uint); !ok {
		return *def, fmt.Errorf("%v value needs to be of type uint.", key)
	}
	return v, nil
}

func (m M) UintSlice(key string) (v []uint, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]uint); !ok {
		return v, fmt.Errorf("%v value needs to be of type []uint.", key)
	}
	return v, nil
}

// Int8 returns the value as a int8 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Int8(key string, def *int8) (v int8, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(int8); !ok {
		return *def, fmt.Errorf("%v value needs to be of type int8.", key)
	}
	return v, nil
}

func (m M) Int8Slice(key string) (v []int8, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]int8); !ok {
		return v, fmt.Errorf("%v value needs to be of type []int8.", key)
	}
	return v, nil
}

// Uint8 returns the value as a uint8 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Uint8(key string, def *uint8) (v uint8, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(uint8); !ok {
		return *def, fmt.Errorf("%v value needs to be of type uint8.", key)
	}
	return v, nil
}

func (m M) Uint8Slice(key string) (v []uint8, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]uint8); !ok {
		return v, fmt.Errorf("%v value needs to be of type []uint8.", key)
	}
	return v, nil
}

// Int16 returns the value as a int16 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Int16(key string, def *int16) (v int16, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(int16); !ok {
		return *def, fmt.Errorf("%v value needs to be of type int16.", key)
	}
	return v, nil
}

func (m M) Int16Slice(key string) (v []int16, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]int16); !ok {
		return v, fmt.Errorf("%v value needs to be of type []int16.", key)
	}
	return v, nil
}

// Uint16 returns the value as a uint16 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Uint16(key string, def *uint16) (v uint16, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(uint16); !ok {
		return *def, fmt.Errorf("%v value needs to be of type uint16.", key)
	}
	return v, nil
}

func (m M) Uint16Slice(key string) (v []uint16, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]uint16); !ok {
		return v, fmt.Errorf("%v value needs to be of type []uint16.", key)
	}
	return v, nil
}

// Int32 returns the value as a int32 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Int32(key string, def *int32) (v int32, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(int32); !ok {
		return *def, fmt.Errorf("%v value needs to be of type int32.", key)
	}
	return v, nil
}

func (m M) Int32Slice(key string) (v []int32, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]int32); !ok {
		return v, fmt.Errorf("%v value needs to be of type []int32.", key)
	}
	return v, nil
}

// Uint32 returns the value as a uint32 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Uint32(key string, def *uint32) (v uint32, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(uint32); !ok {
		return *def, fmt.Errorf("%v value needs to be of type uint32.", key)
	}
	return v, nil
}

func (m M) Uint32Slice(key string) (v []uint32, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]uint32); !ok {
		return v, fmt.Errorf("%v value needs to be of type []uint32.", key)
	}
	return v, nil
}

// Int64 returns the value as a int64 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Int64(key string, def *int64) (v int64, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(int64); !ok {
		return *def, fmt.Errorf("%v value needs to be of type int64.", key)
	}
	return v, nil
}

func (m M) Int64Slice(key string) (v []int64, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]int64); !ok {
		return v, fmt.Errorf("%v value needs to be of type []int64.", key)
	}
	return v, nil
}

// Uint64 returns the value as a uint64 type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) Uint64(key string, def *uint64) (v uint64, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		if def != nil {
			return *def, nil
		}
		return v, fmt.Errorf("%v value is required.", key)
	}
	if v, ok = val.(uint64); !ok {
		return *def, fmt.Errorf("%v value needs to be of type uint64.", key)
	}
	return v, nil
}

func (m M) Uint64Slice(key string) (v []uint64, err error) {
	var val interface{}
	var ok bool
	if val, ok = m[key]; !ok {
		return v, nil
	}
	if v, ok = val.([]uint64); !ok {
		return v, fmt.Errorf("%v value needs to be of type []uint64.", key)
	}
	return v, nil
}
