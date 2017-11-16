package token

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/geom"
)

func assertError(idx int, expErr, gotErr error) (msg string, ok bool) {
	if expErr != gotErr {
		// could be because test.err == nil and err != nil.
		if expErr == nil && gotErr != nil {
			msg = fmt.Sprintf("[%v] unexpected error, Expected nil Got %v", idx, gotErr)
			return msg, false
		}
		if expErr != nil && gotErr == nil {
			msg = fmt.Sprintf("[%v] expected error, Expected %v Got nil", idx, expErr)
			return msg, false
		}
		if expErr.Error() != gotErr.Error() {
			msg = fmt.Sprintf("[%v] did not get correct error value, Expected %v Got %v", idx, expErr, gotErr)
			return msg, false

		}
		return "", false
	}
	if expErr != nil {
		// No need to look at other values, expected an error.
		return "", false
	}
	return "", true
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
		if msg, ok := assertError(idx, test.err, err); !ok {
			if msg != "" {
				t.Error(msg)
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
		pt, err := tt.ParsePoint()
		if msg, ok := assertError(idx, test.err, err); !ok {
			if msg != "" {
				t.Error(msg)
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
			err:   fmt.Errorf("expected to find “(” or “EMPTY”"),
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
