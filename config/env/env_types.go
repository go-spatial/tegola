package env

import (
	"errors"
	"os"
	"regexp"
	"strconv"
)

var TypeError = errors.New("type could not be converted")
var EnvironmentError = errors.New("environment variable not found")

var regex *regexp.Regexp

func init() { regex = regexp.MustCompile(`\${[A-Z]+[A-Z1-9_]*}`) }

func replaceEnvVar(in string) (string, error) {

	// TODO(ear7h): FindAllString
	varName := regex.FindString(in)
	// no env var
	if len(varName) == 0 {
		return in, nil
	}

	// trim the leading '${' and trailing '}'
	varName = varName[2:len(varName)-1]

	// get env var
	envVar := os.Getenv(varName)
	if envVar == "" {
		return "", EnvironmentError
	}

	return regex.ReplaceAllString(in, envVar), nil
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
			break
		}

		boolVal, err = strconv.ParseBool(val)
	case bool:
		boolVal = val
	default:
		err = TypeError
	}

	if err != nil {
		return err
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
			break
		}
	default:
		err = TypeError
	}

	if err != nil {
		return err
	}

	*t = String(stringVal)
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
			break
		}
		uintVal, err = strconv.ParseUint(val, 10, 64)
	case int:
		uintVal = uint64(val)
	case int64:
		uintVal = uint64(val)
	case uint:
		uintVal = uint64(val)
	case uint64:
		uintVal = val
	default:
		err = TypeError
	}

	if err != nil {
		return err
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
			break
		}
		floatVal, err = strconv.ParseFloat(val, 64)
	case float64:
		floatVal = val
	case float32:
		floatVal = float64(val)
	default:
		err = TypeError
	}

	if err != nil {
		return err
	}

	*t = Float(floatVal)
	return nil
}
