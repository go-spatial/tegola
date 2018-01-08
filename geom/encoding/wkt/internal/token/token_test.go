package token

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding/wkt/internal/symbol"
)

func assertError(expErr, gotErr error) (msg, expected, got string, ok bool) {
	if expErr != gotErr {
		// could be because test.err == nil and err != nil.
		if expErr == nil && gotErr != nil {
			return "unexpected", "nil", gotErr.Error(), false
		}
		if expErr != nil && gotErr == nil {
			return "expected error", expErr.Error(), "nil", false
		}
		if expErr.Error() != gotErr.Error() {
			return "did not get correct error value", expErr.Error(), gotErr.Error(), false

		}
		return "", "", "", false
	}
	if expErr != nil {
		// No need to look at other values, expected an error.
		return "", "", "", false
	}
	return "", "", "", true
}

func TestParsePointValue(t *testing.T) {
	type tcase struct {
		input string
		exp   []float64
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		pts, err := tt.parsePointValue()
		if msg, expstr, gotstr, ok := assertError(test.err, err); !ok {
			if msg != "" {
				t.Errorf("[%v] %v, Expected %v Got %v", idx, msg, expstr, gotstr)
			}
			return
		}
		if !reflect.DeepEqual(test.exp, pts) {
			t.Errorf("[%v] did not get correct point values, Expected %v Got %v", idx, test.exp, pts)
		}
	}
	tbltest.Cases(
		tcase{input: "123 123 12", exp: []float64{123, 123, 12}},
		tcase{input: "10.0 -34,", exp: []float64{10.0, -34}},
		tcase{input: "1 ", exp: []float64{1}},
		tcase{input: "1 .0", exp: []float64{1, 0}},
		tcase{input: "1 -.1", exp: []float64{1, -.1}},
		tcase{input: " 1 2 ", exp: []float64{1, 2}},
		tcase{input: "1 .", err: &strconv.NumError{
			Func: "ParseFloat",
			Num:  ".",
			Err:  fmt.Errorf(`invalid syntax`),
		}},
	).Run(fn)

}

func TestParsePointe(t *testing.T) {
	type tcase struct {
		input string
		exp   *geom.Point
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		t.Log("Calling ParsePoint.", idx, test.input)
		pt, err := tt.ParsePoint()
		if msg, expstr, gotstr, ok := assertError(test.err, err); !ok {
			if msg != "" {
				t.Errorf("[%v] %v, Expected %v Got %v", idx, msg, expstr, gotstr)
			}
			return
		}
		if !reflect.DeepEqual(test.exp, pt) {
			t.Errorf("[%v] did not get correct point values, Expected %v Got %v", idx, test.exp, pt)
		}
	}
	tbltest.Cases(
		tcase{
			input: "POINT EMPTY",
		},
		tcase{
			input: "POINT EMPTY ",
		},
		tcase{
			input: "POINT ( 1 2 )",
			exp:   &geom.Point{1, 2},
		},
		tcase{
			input: " POINT ( 1 2 ) ",
			exp:   &geom.Point{1, 2},
		},
		tcase{
			input: " POINT ZM ( 1 2 3 4 ) ",
			exp:   &geom.Point{1, 2},
		},
		tcase{
			input: "POINT 1 2",
			err:   fmt.Errorf("expected to find “(” , “ZM”, “M” or “EMPTY”"),
		},
		tcase{
			input: "POINT ( 1 2",
			err:   fmt.Errorf("expected to find “)”"),
		},
		tcase{
			input: "POINT ( 1 )",
			err:   fmt.Errorf("expected to have at least 2 coordinates in a POINT"),
		},
		tcase{
			input: "POINT ( 1 2 3 4 5 )",
			err:   fmt.Errorf("expected to have no more then 4 coordinates in a POINT"),
		},
	).Run(fn)
}

func Test_ParsePointValue(t *testing.T) {
	type tcase struct {
		input string
		zm    byte

		pt  []float64
		err error
	}
	fn := func(tests map[string]tcase) {
		for name, tc := range tests {
			tc := tc
			t.Run(name, func(t *testing.T) {
				tt := NewT(strings.NewReader(tc.input))
				gpt, err := tt._parsePointValue(tc.zm)
				if msg, expstr, gotstr, ok := assertError(tc.err, err); !ok {
					if msg != "" {
						t.Errorf("[%v] %v, Expected %v Got %v", name, msg, expstr, gotstr)
					}
					return
				}

				if !reflect.DeepEqual(tc.pt, gpt) {
					t.Errorf("[%v] did not get correct point values, Expected %v Got %v", name, tc.pt, gpt)
				}

			})
		}

	}
	fn(map[string]tcase{
		"simple pt1 with lpren": {
			input: "( 10 10 )",
			pt:    []float64{10.0, 10.0},
		},
		"simple pt1 with lpren1": {
			input: "( 10 10 )",
			pt:    []float64{10.0, 10.0},
		},
		"simple M pt1 with lpren": {
			input: "( 10 10 10)",
			pt:    []float64{10.0, 10.0, 10.0},
			zm:    symbol.M,
		},

		"simple M pt1 with lpren1": {
			input: "(10 10 10 )",
			pt:    []float64{10.0, 10.0, 10.0},
			zm:    symbol.M,
		},
		"simple ZM pt1 with lpren": {
			input: "( 10 10 10 10)",
			pt:    []float64{10.0, 10.0, 10.0, 10.0},
			zm:    symbol.ZM,
		},
		"simple ZM pt1 with lpren1": {
			input: "(10 10 10 10)",
			pt:    []float64{10.0, 10.0, 10.0, 10.0},
			zm:    symbol.ZM,
		},
	})

}

func TestParseMultiPointe(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.MultiPoint
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		mpt, err := tt.ParseMultiPoint()
		if msg, expstr, gotstr, ok := assertError(test.err, err); !ok {
			if msg != "" {
				t.Errorf("[%v] %v, Expected %v Got %v", idx, msg, expstr, gotstr)
			}
			return
		}
		if !reflect.DeepEqual(test.exp, mpt) {
			t.Errorf("[%v] did not get correct multipoint values, Expected %v Got %v", idx, test.exp, mpt)
		}

	}
	tbltest.Cases(
		tcase{input: "MultiPoint EMPTY"},
		tcase{
			input: "MULTIPOINT ( 10 10, 12 12 )",
			exp:   geom.MultiPoint{{10, 10}, {12, 12}},
		},
		tcase{
			input: "MULTIPOINT ( (10 10), (12 12) )",
			exp:   geom.MultiPoint{{10, 10}, {12, 12}},
		},
	).Run(fn)
}

func TestParseFloat64(t *testing.T) {
	type tcase struct {
		input string
		exp   float64
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseFloat64()
		if test.err != err {
			t.Errorf("[%v] did not get correct error value, Expected %v Got %v", idx, test.err, err)
		}
		if test.err != nil {
			return
		}
		if test.exp != f {
			t.Errorf("[%v] Exp: %v Got: %v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{input: "-12", exp: -12.0},
		tcase{input: "0", exp: 0.0},
		tcase{input: "+1000.00", exp: 1000.0},
		tcase{input: "-12000.00", exp: -12000.0},
		tcase{input: "10.005e5", exp: 10.005e5},
		tcase{input: "10.005e+5", exp: 10.005e5},
		tcase{input: "10.005e+05", exp: 10.005e5},
		tcase{input: "1.0005e+6", exp: 10.005e5},
		tcase{input: "1.0005e+06", exp: 10.005e5},
		tcase{input: "1.0005e-06", exp: 1.0005e-06},
		tcase{input: "1.0005e-06a", exp: 1.0005e-06},
	).Run(fn)

}
