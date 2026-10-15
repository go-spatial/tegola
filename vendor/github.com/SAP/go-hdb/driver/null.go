package driver

import (
	"reflect"
)

var boolReflectType = reflect.TypeFor[bool]()

// see https://github.com/golang/go/issues/54393
func isGenericNull(t reflect.Type) (bool, reflect.Type) {
	if t.Kind() != reflect.Struct {
		return false, nil
	}

	// quirky check - we don't do and let similar definitions pass as well.
	/*
		if !strings.HasPrefix(t.String(), "sql.Null[") {
			return false
		}
	*/

	var vField reflect.Type
	var validField reflect.Type

	for field := range t.Fields() {
		switch field.Name {
		case "V":
			vField = field.Type
			if validField != nil {
				return true, vField
			}
		case "Valid":
			if field.Type != boolReflectType { // valid needs to be a boolean
				return false, nil
			}
			validField = field.Type
			if vField != nil {
				return true, vField
			}
		}
	}
	return false, nil
}
