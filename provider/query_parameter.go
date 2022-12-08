package provider

import (
	"fmt"
	"strings"
)

// QueryParameter represents an HTTP query parameter specified for use with
// a given map instance.
type QueryParameter struct {
	Name  string `toml:"name"`
	Token string `toml:"token"`
	Type  string `toml:"type"`
	SQL   string `toml:"sql"`
	// DefaultSQL replaces SQL if param wasn't passed. Either default_sql or
	// default_value can be specified
	DefaultSQL   string `toml:"default_sql"`
	DefaultValue string `toml:"default_value"`
}

// Normalize normalizes param and sets default values
func (param *QueryParameter) Normalize() {
	param.Token = strings.ToUpper(param.Token)

	if len(param.SQL) == 0 {
		param.SQL = "?"
	}
}

func (param *QueryParameter) ToValue(rawValue string) (QueryParameterValue, error) {
	val, err := ParamTypeDecoders[param.Type](rawValue)
	if err != nil {
		return QueryParameterValue{}, err
	}
	return QueryParameterValue{
		Token:    param.Token,
		SQL:      param.SQL,
		Value:    val,
		RawParam: param.Name,
		RawValue: rawValue,
	}, nil
}

func (param *QueryParameter) ToDefaultValue() (QueryParameterValue, error) {
	if len(param.DefaultValue) > 0 {
		val, err := ParamTypeDecoders[param.Type](param.DefaultValue)
		return QueryParameterValue{
			Token:    param.Token,
			SQL:      param.SQL,
			Value:    val,
			RawParam: param.Name,
			RawValue: "",
		}, err
	}
	if len(param.DefaultSQL) > 0 {
		return QueryParameterValue{
			Token:    param.Token,
			SQL:      param.DefaultSQL,
			Value:    nil,
			RawParam: param.Name,
			RawValue: "",
		}, nil
	}
	return QueryParameterValue{}, fmt.Errorf("the required parameter %s is not specified", param.Name)
}
