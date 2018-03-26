package dict

import (
	"testing"
	"fmt"
)

func TestMPrimatives(t *testing.T) {
	// go does not allow pointer literals
	fooString := "an eloquent quote"
	fooInt := 8675309

	
	testcases := map[string] struct {
		m M
		test func(m M) (interface{}, error)
		expected interface{}
		err error
	}{
		"string 1" : {
			m: M{"key1": "robpike"},
			test: func(m M) (interface{}, error) {
				def := "default"
				return m.String("key1", &def)
			},
			expected: "robpike",
			err: nil,
		},
		"string 2" : {
			m: M{},
			test: func(m M) (interface{}, error) {
				def := "default"
				return m.String("key1", &def)
			},
			expected: "default",
			err: nil,
		},
		"string ptr 1" : {
			m: M{"key1": &fooString},
			test: func(m M) (interface{}, error) {
				def := "default"
				return m.String("key1", &def)
			},
			expected: fooString,
			err: nil,
		},
		"int 1" : {
			m: M{"key1": 1970},
			test: func(m M) (interface{}, error) {
				def := 2018
				return m.Int("key1", &def)
			},
			expected: 1970,
			err: nil,
		},
		"int ptr 1" : {
			m: M{"key1": &fooInt},
			test: func(m M) (interface{}, error) {
				def := 2018
				return m.Int("key1", &def)
			},
			expected: fooInt,
			err: nil,
		},
		"error 1": {
			m: M{"key1": "stringy"},
			test: func(m M) (interface{}, error) {
				def := 42
				return m.Int("key1", &def)
			},
			expected: 0,
			err: fmt.Errorf("key1 value needs to be of type int. Value is of type string"),
		},

	}

	for k, tc := range testcases {
		res, err := tc.test(tc.m)
		if err != nil && tc.err == nil {
			t.Fatalf("[%v] unexpected error %v", k, err)
		}

		if err == nil && tc.err != nil {
			t.Fatalf("[%v] unexpected return value %v, expected error %v", k, res, tc.err)
		}

		if tc.err != nil {
			if tc.err.Error() != err.Error() {
				t.Fatalf("[%v] unexpected error %v, expected %v", k, err, tc.err)
			}
			continue
		}

		if res != tc.expected {
			t.Fatalf("[%v] incorrect return value expected %v, got %v", k, tc.expected, res)
		}
	}
}