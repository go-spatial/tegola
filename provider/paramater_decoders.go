package provider

import "strconv"

// ParamTypeDecoders is a collection of parsers for different types of user-defined parameters
var ParamTypeDecoders = map[string]func(string) (interface{}, error){
	"int": func(s string) (interface{}, error) {
		return strconv.Atoi(s)
	},
	"float": func(s string) (interface{}, error) {
		return strconv.ParseFloat(s, 32)
	},
	"string": func(s string) (interface{}, error) {
		return s, nil
	},
	"bool": func(s string) (interface{}, error) {
		return strconv.ParseBool(s)
	},
}
