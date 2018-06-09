package dict

import (
	"fmt"
	"reflect"
)

// Dicter is an abstraction over map[string]interface{}
// with the intent of allowing manipulation between the
// underlying data and requests for that data.
type Dicter interface {
	String(key string, Default *string) (string, error)
	StringSlice(key string) ([]string, error)

	Bool(key string, Default *bool) (bool, error)
	BoolSlice(key string) ([]bool, error)

	Int(key string, Default *int) (int, error)
	IntSlice(key string) ([]int, error)

	Uint(key string, Default *uint) (uint, error)
	UintSlice(key string) ([]uint, error)

	Float(key string, Default *float64) (float64, error)
	FloatSlice(key string) ([]float64, error)

	Map(key string) (Dicter, error)
	MapSlice(key string) ([]Dicter, error)

	Interface(key string) (v interface{}, ok bool)

	Keys() []string
}

// ErrKeyRequired is used to communicate a map miss and Dicter implementations should set the value as the missed key
type ErrKeyRequired string

func (err ErrKeyRequired) Error() string {
	return fmt.Sprintf("config: required Key %q not found", string(err))
}

// ErrKeyType is used to communicate the value requested cannot be converted/coerced/manipulated
// according to the method call.
// TODO: rename to ErrType
type ErrKeyType struct {
	Key   string
	Value interface{}
	T     reflect.Type
}

func (err ErrKeyType) Error() string {
	return fmt.Sprintf("config: value mapped to %q is %T not %s", err.Key, err.Value, err.T.String())
}
