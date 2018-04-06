package cmd

import (
	"testing"
	"reflect"
	"fmt"
)

// TODO(ear7h): use internal package, currently in maths/internal
func errOk(expected, got error) bool {
	if expected == nil && got == nil {
		return true
	}

	if expected != nil && got != nil {
		return expected.Error() == got.Error()
	}

	return false
}

func TestNewFormat(t *testing.T) {
	testcases := map[string]struct {
		formatStr string
		exepected Format
		err       error
	}{
		"1": {
			formatStr: "/zxy",
			exepected: Format{1, 2, 0, "/"},
			err:       nil,
		},
		"2": {
			formatStr: " xyz",
			exepected: Format{0, 1, 2, " "},
			err:       nil,
		},
		"3 invalid formatStr": {
			formatStr: "//zxy",
			err:       fmt.Errorf("invalid formatStr //zxy"),
		},
		"4 invalid formatStr": {
			formatStr: "1zxy",
			err:       fmt.Errorf("invalid formatStr 1zxy"),
		},
	}

	for k, tc := range testcases {

		f, err := NewFormat(tc.formatStr)
		if errOk(tc.err, err) {
			continue
		} else {
			t.Errorf("[%v] unexpected err, expected %v got %v", k, tc.err, err)
			continue
		}

		if tc.err == nil && !reflect.DeepEqual(tc.exepected, f) {
			t.Errorf("[%v] expected Format %v got %v", k, tc.exepected, f)
		}
	}
}

func TestFormatParse(t *testing.T) {
	testcases := map[string]struct {
		format  Format
		input   string
		z, x, y uint
		err     error
	}{
		"1": {
			format: Format{1, 2, 0, "/"},
			input:  "0/0/0",
			z:      0,
			x:      0,
			y:      0,
		},
		"2": {
			format: Format{1, 2, 0, "-"},
			input:  "10-2-2",
			z:      10,
			x:      2,
			y:      2,
		},
	}

	for k, tc := range testcases {

		z, x, y, err := tc.format.Parse(tc.input)
		if errOk(tc.err, err) {
			continue
		} else {
			t.Errorf("[%v] unexpected err, expected %v got %v", k, tc.err, err)
			continue
		}

		if z != tc.z || x != tc.x || y != tc.y {
			t.Errorf("[%v] expected output (z:%v, x:%v, y:%v) got (z:%v, x:%v, y:%v)", k, tc.z, tc.x, tc.y, z, x, y)
		}
	}
}
