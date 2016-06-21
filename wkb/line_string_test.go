package wkb_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/terranodo/tegola/wkb"
)

func cmpLines(l1, l2 wkb.LineString) (bool, string) {

	if len(l1) != len(l2) {
		return false, fmt.Sprintf(
			"Lengths of lines do not match.\n"+
				"Line 1 has %v points, Line 2 has %v points.\n",
			len(l1),
			len(l2),
		)
	}
	for i := range l1 {
		if l1[i].X() != l2[i].X() {
			return false, fmt.Sprintf("Point %v x coordinate does not match. (%v != %v)", i+1, l1[i].X(), l2[i].X())
		}
		if l1[i].Y() != l2[i].Y() {
			return false, fmt.Sprintf("Point %v y coordinate does not match.(%v != %v)", i+1, l1[i].Y(), l2[i].Y())
		}
	}
	return true, ""
}

func TestLineString(t *testing.T) {
	testcases := TestCases{
		{
			bytes: []byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 240, 63, 0, 0, 0, 0, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0, 8, 64, 0, 0, 0, 0, 0, 0, 16, 64},
			bom:   binary.LittleEndian,
			expected: &wkb.LineString{
				wkb.NewPoint(1, 2),
				wkb.NewPoint(3, 4),
			},
		},
	}

	var ls, expected wkb.LineString
	testcases.RunTests(t, func(num int, tcase *TestCase) {
		if cexp, ok := tcase.expected.(*wkb.LineString); !ok {
			t.Errorf("Bad test case %v", num)
			return
		} else {
			expected = *cexp
		}
		if err := ls.Decode(tcase.bom, tcase.Reader()); err != nil {
			t.Errorf("Got unexpected error %v", err)
			return
		}
		if ok, err := cmpLines(expected, ls); !ok {
			t.Errorf("Failed LineString Test: %v:%v\n %v", num, tcase.bytes, err)
		}
	})
}

func TestMultiLineString(t *testing.T) {
	testcases := TestCases{
		{
			bytes: []byte{
				0x02, 0x00, 0x00, 0x00, // Number of Lines 2
				0x01,                   // Encoding Little
				0x02, 0x00, 0x00, 0x00, // Type
				0x03, 0x00, 0x00, 0x00, // Number of points 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, // X1 10
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, // Y1 10
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x34, 0x40, // X2 20
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x34, 0x40, // Y2 20
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, // X3 10
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x40, // Y3 40
				0x01,                   // Encoding Little
				0x02, 0x00, 0x00, 0x00, // Type LineString
				0x04, 0x00, 0x00, 0x00, // Number of Points 4
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x40, // X1 40
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x40, // Y1 40
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3e, 0x40, // X2 30
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3e, 0x40, // Y2 40
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x40, // X3 40
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x34, 0x40, // Y3 20
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3e, 0x40, // X4 30
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, // Y4 10
			},
			bom: binary.LittleEndian,
			expected: &wkb.MultiLineString{
				wkb.LineString{wkb.NewPoint(10, 10), wkb.NewPoint(20, 20), wkb.NewPoint(10, 40)},
				wkb.LineString{wkb.NewPoint(40, 40), wkb.NewPoint(30, 30), wkb.NewPoint(40, 20), wkb.NewPoint(30, 10)},
			},
		},
	}

	testcases.RunTests(t, func(num int, tcase *TestCase) {
		var ls, expected wkb.MultiLineString
		if cexp, ok := tcase.expected.(*wkb.MultiLineString); !ok {
			t.Errorf("Bad test case %v", num)
			return
		} else {
			expected = *cexp
		}
		if err := ls.Decode(tcase.bom, tcase.Reader()); err != nil {
			t.Errorf("Got unexpected error %v", err)
			return
		}
		if len(ls) != len(expected) {
			t.Errorf(
				"Failed MultiLineString Test %v: %v "+
					"Number of expected lines is wrong. "+
					"Expected %v, Got %v\n "+
					"Expected lines are:\n%v\n"+
					"Lines gotten:\n%v\n",
				num,
				tcase.bytes,
				len(expected),
				len(ls),
				expected,
				ls,
			)
			return
		}
		for j := range ls {
			if ok, err := cmpLines(expected[j], ls[j]); !ok {
				t.Errorf("Failed MultiLineString Test(%v) for lines (%v -- %v): %v\n%v ", num, expected[j], ls[j], tcase.bytes, err)
				return
			}
		}
	})
}
