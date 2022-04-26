package env

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// evnVarRegex matches a variable surrounded by curly braces with leading dollar sign.
// ex: ${MY_VAR}
var envVarRegex = regexp.MustCompile(`\${[A-Z]+[A-Z1-9_]*}`)

// ErrEnvVar corresponds with a missing environment variable
type ErrEnvVar string

func (e ErrEnvVar) Error() string {
	return fmt.Sprintf("environment variable %q not found", string(e))
}

// ErrType corresponds with an incorrect type passed to UnmarshalTOML
type ErrType struct {
	v interface{}
}

func (te ErrType) Error() string {
	return fmt.Sprintf("type %t could not be converted", te.v)
}

// replaceEnvVars replaces environment variable placeholders in reader stream with values
func replaceEnvVar(in string) (string, error) {
	// loop through all environment variable matches
	for locs := envVarRegex.FindStringIndex(in); locs != nil; locs = envVarRegex.FindStringIndex(in) {

		// extract match from the input string
		match := in[locs[0]:locs[1]]

		// trim the leading '${' and trailing '}'
		varName := match[2 : len(match)-1]

		// get env var
		envVar, ok := os.LookupEnv(varName)
		if !ok {
			return "", ErrEnvVar(varName)
		}

		// update the input string with the env values
		in = strings.Replace(in, match, envVar, -1)
	}

	return in, nil
}

//TODO(@ear7h): implement UnmarshalJSON for types

func (t *Dict) UnmarshalTOML(v interface{}) error {
	var d *Dict
	var err error

	d, err = ParseDict(v)
	if err != nil {
		return err
	}

	*t = *d

	return nil
}

type Bool bool

func BoolPtr(v Bool) *Bool {
	return &v
}

func (t *Bool) UnmarshalTOML(v interface{}) error {
	var b *bool
	var err error

	b, err = ParseBool(v)
	if err != nil {
		return err
	}

	*t = Bool(*b)
	return nil
}

type String string

func StringPtr(v String) *String {
	return &v
}

func (t *String) UnmarshalTOML(v interface{}) error {
	var s *string
	var err error

	s, err = ParseString(v)
	if err != nil {
		return err
	}

	*t = String(*s)
	return nil
}

type Int int

func IntPtr(v Int) *Int {
	return &v
}

func (t *Int) UnmarshalTOML(v interface{}) error {
	var i *int
	var err error

	i, err = ParseInt(v)
	if err != nil {
		return err
	}

	*t = Int(*i)
	return nil
}

type Uint uint

func UintPtr(v Uint) *Uint {
	return &v
}

func (t *Uint) UnmarshalTOML(v interface{}) error {
	var ui *uint
	var err error

	ui, err = ParseUint(v)
	if err != nil {
		return err
	}

	*t = Uint(*ui)
	return nil
}

type Float float64

func FloatPtr(v Float) *Float {
	return &v
}

func (t *Float) UnmarshalTOML(v interface{}) error {
	var f *float64
	var err error

	f, err = ParseFloat(v)
	if err != nil {
		return err
	}

	*t = Float(*f)
	return nil
}
