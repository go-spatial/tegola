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
		formatStr     string
		exepected     Format
		isValidFormat bool
	}{
		"1": {
			formatStr:     "/zxy",
			exepected:     Format{1, 2, 0, "/"},
			isValidFormat: true,
		},
		"2": {
			formatStr:     " xyz",
			exepected:     Format{0, 1, 2, " "},
			isValidFormat: true,
		},
		"invalid formatStr 1": {
			formatStr:     "//zxy",
			isValidFormat: false,
		},
		"invalid formatStr 2": {
			formatStr:     "1zxy",
			isValidFormat: false,
		},
		"invalid formatStr 3" : {
			formatStr:     "/1xy",
			isValidFormat: false,
		},
		"invalid formatStr 4" : {
			formatStr:     ",z45",
			isValidFormat: false,
		},
		"invalid formatStr 5" : {
			formatStr:     "zzxy",
			isValidFormat: false,
		},
		"invalid formatStr 6" : {
			formatStr:     "$xxx",
			isValidFormat: false,
		},
		"invalid formatStr 7" : {
			formatStr:     "$xyx",
			isValidFormat: false,
		},
		"invalid formatStr 8" : {
			formatStr:     "$$$$",
			isValidFormat: false,
		},
		"invalid formatStr 9" : {
			formatStr:     ",100",
			isValidFormat: false,
		},
	}

	for k, tc := range testcases {

		f, err := NewFormat(tc.formatStr)
		// error must be nil with valid formats
		// and not nil with invalid formats
		if (err != nil) == tc.isValidFormat {
			// ErrTileNameFormat should be the only error
			var exerr error
			if tc.isValidFormat {
				exerr = ErrTileNameFormat(tc.formatStr)
			}
			t.Errorf("[%v] unexpected err, expected %v got %v", k, exerr, err)
			continue
		}

		if tc.isValidFormat && !reflect.DeepEqual(tc.exepected, f) {
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
		"invalid input separators": {
			format: Format{1, 2, 0, "-"},
			input:  "102-2",
			err: fmt.Errorf("invalid zxy value (%v). expecting the formatStr %v", "102-2", Format{1, 2, 0, "-"}),
		},
		"invalid input z": {
			format: Format{1, 2, 0, "-"},
			input:  "#-2-2",
			err: fmt.Errorf("invalid Z value (%v)", "#"),
		},
		"invalid input z float": {
			format: Format{1, 2, 0, "-"},
			input:  "10.1-2-2",
			err: fmt.Errorf("invalid Z value (%v)", "10.1"),
		},
		"invalid input z too high": {
			format: Format{1, 2, 0, "-"},
			input:  "1000-2-2",
			err: fmt.Errorf("invalid Z value (%v)", "1000"),
		},
		"invalid input x": {
			format: Format{1, 2, 0, "-"},
			input:  "10-#-2",
			err: fmt.Errorf("invalid X value (%v)", "#"),
		},
		"invalid input x too high": {
			format: Format{1, 2, 0, "-"},
			input:  "3-10000-2",
			err: fmt.Errorf("invalid X value (%v)", "10000"),
		},
		"invalid input y": {
			format: Format{1, 2, 0, "-"},
			input:  "10-2-#",
			err: fmt.Errorf("invalid Y value (%v)", "#"),
		},
		"invalid input y too high": {
			format: Format{1, 2, 0, "-"},
			input:  "3-2-10000",
			err: fmt.Errorf("invalid Y value (%v)", "10000"),
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
