package env

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// matches a variable surrounded by curly braces with leading dollar sign.
// ex: ${MY_VAR}
var regex = regexp.MustCompile(`\${[A-Z]+[A-Z1-9_]*}`)

// EnvironmentError corresponds with a missing environment variable
type EnvironmentError struct {
	varName string
}

func (ee EnvironmentError) Error() string {
	return fmt.Sprintf("environment variable %q not found", ee.varName)
}

// TypeError corresponds with an incorrect type passed to UnmarshalTOML
type TypeError struct {
	v interface{}
}

func (te *TypeError) Error() string {
	return fmt.Sprintf("type %t could not be converted", te.v)
}

// replaceEnvVars replaces environment variable placeholders in reader stream with values
func replaceEnvVar(in string) (string, error) {
	// loop through all environment variable matches
	for locs := regex.FindStringIndex(in); locs != nil; locs = regex.FindStringIndex(in) {

		// extract match from the input string
		match := in[locs[0]:locs[1]]

		// trim the leading '${' and trailing '}'
		varName := match[2 : len(match)-1]

		// get env var
		envVar, ok := os.LookupEnv(varName)
		if !ok {
			return "", &EnvironmentError{varName: varName}
		}

		// update the input string with the env values
		in = strings.Replace(in, match, envVar, -1)
	}

	return in, nil
}

type Bool bool

func BoolPtr(v Bool) *Bool {
	return &v
}

func (t *Bool) UnmarshalTOML(v interface{}) error {
	var boolVal bool
	var err error

	switch val := v.(type) {
	case string:
		val, err = replaceEnvVar(val)
		if err != nil {
			return err
		}

		boolVal, err = strconv.ParseBool(val)
		if err != nil {
			return err
		}
	case bool:
		boolVal = val
	default:
		return &TypeError{v}
	}

	*t = Bool(boolVal)
	return nil
}

type String string

func StringPtr(v String) *String {
	return &v
}

func (t *String) UnmarshalTOML(v interface{}) error {
	var stringVal string
	var err error

	switch val := v.(type) {
	case string:
		stringVal, err = replaceEnvVar(val)
		if err != nil {
			return err
		}
	default:
		return &TypeError{v}
	}

	*t = String(stringVal)
	return nil
}

type Int int

func IntPtr(v Int) *Int {
	return &v
}

func (t *Int) UnmarshalTOML(v interface{}) error {
	var intVal int64
	var err error

	switch val := v.(type) {
	case string:
		val, err = replaceEnvVar(val)
		if err != nil {
			return err
		}
		intVal, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
	case int:
		intVal = int64(val)
	case int64:
		intVal = val
	case uint:
		intVal = int64(val)
	case uint64:
		intVal = int64(val)
	default:
		return &TypeError{v}
	}

	*t = Int(intVal)
	return nil
}

type Uint uint

func UintPtr(v Uint) *Uint {
	return &v
}

func (t *Uint) UnmarshalTOML(v interface{}) error {
	var uintVal uint64
	var err error

	switch val := v.(type) {
	case string:
		val, err = replaceEnvVar(val)
		if err != nil {
			return err
		}
		uintVal, err = strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
	case int:
		uintVal = uint64(val)
	case int64:
		uintVal = uint64(val)
	case uint:
		uintVal = uint64(val)
	case uint64:
		uintVal = val
	default:
		return &TypeError{v}
	}

	*t = Uint(uintVal)
	return nil
}

type Float float64

func FloatPtr(v Float) *Float {
	return &v
}
func (t *Float) UnmarshalTOML(v interface{}) error {
	var floatVal float64
	var err error

	switch val := v.(type) {
	case string:
		val, err = replaceEnvVar(val)
		if err != nil {
			return err
		}
		floatVal, err = strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
	case float64:
		floatVal = val
	case float32:
		floatVal = float64(val)
	default:
		return &TypeError{v}
	}

	*t = Float(floatVal)
	return nil
}
