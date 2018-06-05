package env_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/internal/env"
)

func TestDict(t *testing.T) {

	type tcase struct {
		dict        env.Dict
		key         string
		envVars     map[string]string
		expected    interface{}
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {
		// setup our env vars
		for k, v := range tc.envVars {
			os.Setenv(k, v)
		}

		// clean up env vars
		defer (func() {
			for k, _ := range tc.envVars {
				os.Unsetenv(k)
			}
		})()

		var val interface{}
		var err error

		switch tc.expected.(type) {
		case string:
			val, err = tc.dict.String(tc.key, nil)
		case []string:
			val, err = tc.dict.StringSlice(tc.key)
		case bool:
			val, err = tc.dict.Bool(tc.key, nil)
		case []bool:
			val, err = tc.dict.BoolSlice(tc.key)
		case int:
			val, err = tc.dict.Int(tc.key, nil)
		case []int:
			val, err = tc.dict.IntSlice(tc.key)
		case uint:
			val, err = tc.dict.Uint(tc.key, nil)
		case []uint:
			val, err = tc.dict.UintSlice(tc.key)
		case float32, float64:
			val, err = tc.dict.Float(tc.key, nil)
		case []float64:
			val, err = tc.dict.FloatSlice(tc.key)
		case nil:
			// ignore, used for checking errors
		default:
			t.Errorf("invalid type: %T", tc.expected)
			return
		}

		if tc.expectedErr != nil {
			if err == nil {
				t.Errorf("expected err %v, got nil", tc.expectedErr.Error())
				return
			}

			// compare error messages
			if tc.expectedErr.Error() != err.Error() {
				t.Errorf("invalid error. expected %v, got %v", tc.expectedErr, err)
				return
			}

			return
		}
		if err != nil {
			t.Errorf("unexpected err: %v", err)
			return
		}

		if !reflect.DeepEqual(val, tc.expected) {
			t.Errorf("expected %v, got %v", tc.expected, val)
			return
		}
	}

	tests := map[string]tcase{
		"string": {
			dict: env.Dict{
				"string": "${TEST_STRING}",
			},
			envVars: map[string]string{
				"TEST_STRING": "foo",
			},
			key:      "string",
			expected: "foo",
		},
		"string no env": {
			dict: env.Dict{
				"string": "foo",
			},
			key:      "string",
			expected: "foo",
		},
		"string env not set": {
			dict: env.Dict{
				"string": "${TEST_STRING}",
			},
			key:         "string",
			expected:    "",
			expectedErr: env.ErrEnvVar("TEST_STRING"),
		},
		"string slice": {
			dict: env.Dict{
				"string_slice": "${TEST_STRING}",
			},
			envVars: map[string]string{
				"TEST_STRING": "foo, bar",
			},
			key:      "string_slice",
			expected: []string{"foo", "bar"},
		},
		"string slice no env": {
			dict: env.Dict{
				"string_slice": []string{"foo", "bar", "baz"},
			},
			key:      "string_slice",
			expected: []string{"foo", "bar", "baz"},
		},
		"string slice concat no env": {
			dict: env.Dict{
				"string_slice": "foo, bar,  baz",
			},
			key:      "string_slice",
			expected: []string{"foo", "bar", "baz"},
		},
		"string slice env not set": {
			dict: env.Dict{
				"string_slice": "${TEST_STRING}",
			},
			key:         "string_slice",
			expected:    []string{""},
			expectedErr: env.ErrEnvVar("TEST_STRING"),
		},
		"bool": {
			dict: env.Dict{
				"bool": "${TEST_BOOL}",
			},
			envVars: map[string]string{
				"TEST_BOOL": "true",
			},
			key:      "bool",
			expected: true,
		},
		"bool no env": {
			dict: env.Dict{
				"bool": true,
			},
			key:      "bool",
			expected: true,
		},
		"bool env not set": {
			dict: env.Dict{
				"bool": "${TEST_BOOL}",
			},
			key:         "bool",
			expected:    true,
			expectedErr: env.ErrEnvVar("TEST_BOOL"),
		},
		"bool slice": {
			dict: env.Dict{
				"bool_slice": "${TEST_BOOL}",
			},
			envVars: map[string]string{
				"TEST_BOOL": "true, false",
			},
			key:      "bool_slice",
			expected: []bool{true, false},
		},
		"bool slice no env": {
			dict: env.Dict{
				"bool_slice": []bool{true, false, true},
			},
			key:      "bool_slice",
			expected: []bool{true, false, true},
		},
		"bool slice concat no env": {
			dict: env.Dict{
				"bool_slice": "true, false,  true",
			},
			key:      "bool_slice",
			expected: []bool{true, false, true},
		},
		"bool slice env not set": {
			dict: env.Dict{
				"bool_slice": "${TEST_BOOL}",
			},
			key:         "bool_slice",
			expected:    []bool{true},
			expectedErr: env.ErrEnvVar("TEST_BOOL"),
		},
		"int": {
			dict: env.Dict{
				"int": "${TEST_INT}",
			},
			envVars: map[string]string{
				"TEST_INT": "-1",
			},
			key:      "int",
			expected: -1,
		},
		"int no env": {
			dict: env.Dict{
				"int": -1,
			},
			key:      "int",
			expected: -1,
		},
		"int env not set": {
			dict: env.Dict{
				"int": "${TEST_INT}",
			},
			key:         "int",
			expected:    -1,
			expectedErr: env.ErrEnvVar("TEST_INT"),
		},
		"int slice": {
			dict: env.Dict{
				"int_slice": "${TEST_INT_SLICE}",
			},
			envVars: map[string]string{
				"TEST_INT_SLICE": "123, -324",
			},
			key:      "int_slice",
			expected: []int{123, -324},
		},
		"int slice no env": {
			dict: env.Dict{
				"int_slice": []int{43, -23, 12},
			},
			key:      "int_slice",
			expected: []int{43, -23, 12},
		},
		"int slice concat no env": {
			dict: env.Dict{
				"int_slice": "43, -23, 12",
			},
			key:      "int_slice",
			expected: []int{43, -23, 12},
		},
		"int slice env not set": {
			dict: env.Dict{
				"int_slice": "${TEST_INT_SLICE}",
			},
			key:         "int_slice",
			expected:    []int{0},
			expectedErr: env.ErrEnvVar("TEST_INT_SLICE"),
		},
		"uint": {
			dict: env.Dict{
				"uint": "${TEST_UINT}",
			},
			envVars: map[string]string{
				"TEST_UINT": "1",
			},
			key:      "uint",
			expected: uint(1),
		},
		"uint no env": {
			dict: env.Dict{
				"uint": uint(1),
			},
			key:      "uint",
			expected: uint(1),
		},
		"uint env not set": {
			dict: env.Dict{
				"uint": "${TEST_UINT}",
			},
			key:         "uint",
			expected:    uint(1),
			expectedErr: env.ErrEnvVar("TEST_UINT"),
		},
		"uint slice": {
			dict: env.Dict{
				"uint_slice": "${TEST_UINT_SLICE}",
			},
			envVars: map[string]string{
				"TEST_UINT_SLICE": "123, 324",
			},
			key:      "uint_slice",
			expected: []uint{123, 324},
		},
		"uint slice no env": {
			dict: env.Dict{
				"uint_slice": []uint{43, 23, 12},
			},
			key:      "uint_slice",
			expected: []uint{43, 23, 12},
		},
		"uint slice concat no env": {
			dict: env.Dict{
				"uint_slice": "43, 23, 12",
			},
			key:      "uint_slice",
			expected: []uint{43, 23, 12},
		},
		"uint slice env not set": {
			dict: env.Dict{
				"uint_slice": "${TEST_UINT_SLICE}",
			},
			key:         "uint_slice",
			expected:    []uint{0},
			expectedErr: env.ErrEnvVar("TEST_UINT_SLICE"),
		},
		"float": {
			dict: env.Dict{
				"float": "${TEST_FLOAT}",
			},
			envVars: map[string]string{
				"TEST_FLOAT": "1.0",
			},
			key:      "float",
			expected: 1.0,
		},
		"float no env": {
			dict: env.Dict{
				"float": 1.0,
			},
			key:      "float",
			expected: 1.0,
		},
		"float env not set": {
			dict: env.Dict{
				"float": "${TEST_FLOAT}",
			},
			key:         "float",
			expected:    1.0,
			expectedErr: env.ErrEnvVar("TEST_FLOAT"),
		},
		"float slice": {
			dict: env.Dict{
				"float_slice": "${TEST_FLOAT_SLICE}",
			},
			envVars: map[string]string{
				"TEST_FLOAT_SLICE": "123.0, 324.0",
			},
			key:      "float_slice",
			expected: []float64{123.0, 324.0},
		},
		"float slice no env": {
			dict: env.Dict{
				"float_slice": []float64{43.0, 23.0, 12.0},
			},
			key:      "float_slice",
			expected: []float64{43.0, 23.0, 12.0},
		},
		"float slice concat no env": {
			dict: env.Dict{
				"float_slice": "43.0, 23.0, 12.0",
			},
			key:      "float_slice",
			expected: []float64{43.0, 23.0, 12.0},
		},
		"float slice env not set": {
			dict: env.Dict{
				"float_slice": "${TEST_FLOAT_SLICE}",
			},
			key:         "float_slice",
			expected:    []float64{0.0},
			expectedErr: env.ErrEnvVar("TEST_FLOAT_SLICE"),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
