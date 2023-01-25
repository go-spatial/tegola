package driver

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
)

// ErrEndOfRows is the error to be returned using a function based bulk exec to indicate
// the end of rows.
var ErrEndOfRows = errors.New("end of rows")

type argsScanner interface {
	scan(nvargs []driver.NamedValue) error
}

type singleArgs struct {
	i      int
	nvargs []driver.NamedValue
}

func (it *singleArgs) scan(nvargs []driver.NamedValue) error {
	if it.i != 0 {
		return ErrEndOfRows
	}
	copy(nvargs, it.nvargs)
	it.i++
	return nil
}

type multiArgs struct {
	i      int
	nvargs []driver.NamedValue
}

func (it *multiArgs) scan(nvargs []driver.NamedValue) error {
	if it.i >= len(it.nvargs) {
		return ErrEndOfRows
	}
	n := copy(nvargs, it.nvargs[it.i:])
	it.i += n
	return nil
}

type fctArgs struct {
	fct  func(args []any) error
	args []any
}

func (it *fctArgs) scan(nvargs []driver.NamedValue) error {
	if it.args == nil {
		it.args = make([]any, len(nvargs))
	}
	err := it.fct(it.args)
	if err != nil {
		return err
	}
	for i := 0; i < len(nvargs); i++ {
		nvargs[i] = convertToNamedValue(i, it.args[i])
	}
	return nil
}

func convertToNamedValue(idx int, arg any) driver.NamedValue {
	switch t := arg.(type) {
	case sql.NamedArg:
		return driver.NamedValue{Name: t.Name, Ordinal: idx + 1, Value: t.Value}
	default:
		return driver.NamedValue{Ordinal: idx + 1, Value: arg}
	}
}

type anyListArgs struct {
	i    int
	list []any
}

func (it *anyListArgs) scan(nvargs []driver.NamedValue) error {
	if it.i >= len(it.list) {
		return ErrEndOfRows
	}
	nvargs[0] = convertToNamedValue(0, it.list[it.i])
	it.i++
	return nil
}

type anyTableArgs struct {
	i     int
	table [][]any
}

func (it *anyTableArgs) scan(nvargs []driver.NamedValue) error {
	if it.i >= len(it.table) {
		return ErrEndOfRows
	}
	for j := 0; j < len(nvargs); j++ {
		nvargs[j] = convertToNamedValue(j, it.table[it.i][j])
	}
	it.i++
	return nil
}

type genListArgs struct {
	i    int
	list reflect.Value
}

func (it *genListArgs) scan(nvargs []driver.NamedValue) error {
	if it.i >= it.list.Len() {
		return ErrEndOfRows
	}
	nvargs[0] = convertToNamedValue(0, it.list.Index(it.i).Interface())
	it.i++
	return nil
}

type genTableArgs struct {
	i     int
	table reflect.Value
}

func (it *genTableArgs) scan(nvargs []driver.NamedValue) error {
	if it.i >= it.table.Len() {
		return ErrEndOfRows
	}
	list := it.table.Index(it.i)
	for j := 0; j < len(nvargs); j++ {
		nvargs[j] = convertToNamedValue(j, list.Index(j).Interface())
	}
	it.i++
	return nil
}

func isList(v any) (any, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		// but do not allow slice, array of bytes
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return nil, false
		}
		return rv.Interface(), true
	case reflect.Ptr:
		return isList(rv.Elem().Interface())
	default:
		return nil, false
	}
}

func isTable(v any) (any, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if list, ok := isList(rv.Elem().Interface()); ok {
			return list, true
		}
		return nil, false
	case reflect.Ptr:
		return isTable(rv.Elem().Interface())
	default:
		return nil, false
	}
}

type argsMismatchError struct {
	numArg int
	numPrm int
}

func newArgsMismatchError(numArg, numPrm int) *argsMismatchError {
	return &argsMismatchError{numArg: numArg, numPrm: numPrm}
}

func (e *argsMismatchError) Error() string {
	return fmt.Sprintf("argument parameter mismatch - number of arguments %d number of parameters %d", e.numArg, e.numPrm)
}

func newArgsScanner(numField int, nvargs []driver.NamedValue, legacy bool) (argsScanner, error) {
	numArg := len(nvargs)

	switch numArg {

	case 0:
		if numField == 0 {
			return nil, nil
		}
		return nil, newArgsMismatchError(numArg, numField)

	case 1:
		arg := nvargs[0].Value

		switch numField {
		case 0:
			return nil, newArgsMismatchError(numArg, numField)
		case 1:
			if v, ok := arg.(func(args []any) error); ok {
				return &fctArgs{fct: v}, nil
			}
			if v, ok := arg.([]any); ok {
				if !legacy {
					return nil, errBulkExecDeprecated
				}
				return &anyListArgs{list: v}, nil
			}
			if v, ok := isList(arg); ok {
				if !legacy {
					return nil, errBulkExecDeprecated
				}
				return &genListArgs{list: reflect.ValueOf(v)}, nil
			}
			return &singleArgs{nvargs: nvargs}, nil
		default:
			if v, ok := arg.(func(args []any) error); ok {
				return &fctArgs{fct: v}, nil
			}
			if v, ok := arg.([][]any); ok {
				if !legacy {
					return nil, errBulkExecDeprecated
				}
				return &anyTableArgs{table: v}, nil
			}
			if v, ok := isTable(arg); ok {
				if !legacy {
					return nil, errBulkExecDeprecated
				}
				return &genTableArgs{table: reflect.ValueOf(v)}, nil
			}
			return nil, fmt.Errorf("invalid argument type %T", arg)
		}

	default:
		if numField == 0 {
			return nil, newArgsMismatchError(numArg, numField)
		}
		switch {
		case numArg == numField:
			return &singleArgs{nvargs: nvargs}, nil
		case numArg%numField == 0:
			return &multiArgs{nvargs: nvargs}, nil
		default:
			return nil, newArgsMismatchError(numArg, numField)
		}
	}
}
