package wkb_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/terranodo/tegola/wkb"
)

func cmpPoint(p1, p2 wkb.Point) (bool, string) {
	if p1.X() != p2.X() {
		return false, fmt.Sprintf(" X value of points do not match.p1:(%v) p2:(%v)", p1, p2)
	}
	if p1.Y() != p2.Y() {
		return false, fmt.Sprintf(" Y value of points do not match.p1:(%v) p2:(%v)", p1, p2)
	}
	return true, ""
}

func TestPoint(t *testing.T) {
	testcases := []struct {
		bytes    []byte
		bom      binary.ByteOrder
		expected wkb.Point
	}{
		{
			bytes: []byte{
				//01    02    03    04    05    06    07    08
				0x46, 0x81, 0xF6, 0x23, 0x2E, 0x4A, 0x5D, 0xC0,
				0x03, 0x46, 0x1B, 0x3C, 0xAF, 0x5B, 0x40, 0x40,
			},
			bom:      binary.LittleEndian,
			expected: wkb.NewPoint(-117.15906619141342, 32.71628524142945),
		},
	}

	var p wkb.Point
	for i, test := range testcases {
		buf := bytes.NewReader(test.bytes)
		p.Decode(test.bom, buf)
		if ok, err := cmpPoint(test.expected, p); !ok {
			t.Errorf("Failed Test %v: %v", i, err)
		}
	}
}

func TestMultiPoint(t *testing.T) {
	testcases := []struct {
		bytes    []byte
		bom      binary.ByteOrder
		expected wkb.MultiPoint
	}{
		{
			bytes: []byte{
				0x04, 0x00, 0x00, 0x00, // Number of Points (4)
				0x01,                   // Byte Order Little
				0x01, 0x00, 0x00, 0x00, // Type Point (1)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, // X1 10
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x40, // Y1 40
				0x01,                   // Byte Order Little
				0x01, 0x00, 0x00, 0x00, // Type Point (1)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x44, 0x40, // X2 40
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3e, 0x40, // Y2 30
				0x01,                   // Byte Order Little
				0x01, 0x00, 0x00, 0x00, // Type Point (1)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x34, 0x40, // X3 20
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x34, 0x40, // Y3 20
				0x01,                   // Byte Order Little
				0x01, 0x00, 0x00, 0x00, // Type Point (1)
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3e, 0x40, // X4 30
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x40, // Y4 10
			},
			bom: binary.LittleEndian,
			expected: wkb.MultiPoint{
				wkb.NewPoint(10, 40),
				wkb.NewPoint(40, 30),
				wkb.NewPoint(20, 20),
				wkb.NewPoint(30, 10),
			},
		},
	}

	var p wkb.MultiPoint
	for i, test := range testcases {
		buf := bytes.NewReader(test.bytes)
		err := p.Decode(test.bom, buf)
		if len(test.expected) != len(p) || err != nil {
			t.Errorf("Failed test %v: Not the same number of points, Expected: %v, Got: %v -- err: %v", i, len(test.expected), len(p), err)
			continue
		}
		for j, ep := range test.expected {
			if ok, err := cmpPoint(ep, p[j]); !ok {
				t.Errorf("Failed Test %v: To match point %v, %v", i, err)
			}

		}
	}

}
