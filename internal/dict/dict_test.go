package dict_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/internal/dict"
)

func TestDict(t *testing.T) {

	type tcase struct {
		dict        dict.Dict
		key         string
		expected    interface{}
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {

		var val interface{}
		var err error

		switch tc.expected.(type) {
		case string:
			val, err = tc.dict.String(tc.key, nil)
		case bool:
			val, err = tc.dict.Bool(tc.key, nil)
		case int:
			val, err = tc.dict.Int(tc.key, nil)
		case uint:
			val, err = tc.dict.Uint(tc.key, nil)
		case float32, float64:
			val, err = tc.dict.Float(tc.key, nil)
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
			dict: dict.Dict{
				"host": "foo",
			},
			key:      "host",
			expected: "foo",
		},
		"string error": {
			dict: dict.Dict{
				"host": 1,
			},
			key:      "host",
			expected: "",
			expectedErr: dict.ErrKeyType{
				Key:   "host",
				Value: 1,
				T:     reflect.TypeOf(""),
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
