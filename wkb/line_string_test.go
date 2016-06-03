package wkb_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/terranodo/tegola/wkb"
)

func TestLineString(t *testing.T) {
	testcases := []struct {
		bytes    []byte
		bom      binary.ByteOrder
		expected wkb.LineString
	}{
		{
			bytes: []byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 240, 63, 0, 0, 0, 0, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0, 8, 64, 0, 0, 0, 0, 0, 0, 16, 64},
			bom:   binary.LittleEndian,
			expected: wkb.LineString{
				wkb.Point{
					X: 1,
					Y: 2,
				},
				wkb.Point{
					X: 3,
					Y: 4,
				},
			},
		},
	}

	var ls wkb.LineString
	for i, test := range testcases {
		buf := bytes.NewReader(test.bytes)
		if err := ls.Decode(test.bom, buf); err != nil {
			t.Errorf("Got unexpected error %v", err)
			continue
		}
		if len(ls) != len(test.expected) {
			t.Errorf(
				"Failed Test %v: %v "+
					"Number of expected points is wrong. "+
					"Expected %v, Got %v\n "+
					"Expected Points are:\n%v\n"+
					"Points gotten:\n%v\n",
				i,
				test.bytes,
				len(test.expected),
				len(ls),
				test.expected,
				ls,
			)
		} else {
			for j := range ls {
				if ls[j].X != test.expected[j].X ||
					ls[j].Y != test.expected[j].Y {
					t.Errorf(
						"Failed test %v, For Point(%v) Expected (%v,%v) Got (%v,%v)",
						i,
						j,
						test.expected[j].X,
						test.expected[j].Y,
						ls[j].X,
						ls[j].Y,
					)
				}
			}
		}
	}
}
