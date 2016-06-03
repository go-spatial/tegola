package wkb_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/terranodo/tegola/wkb"
)

func TestPoint(t *testing.T) {
	testcases := []struct {
		bytes    []byte
		bom      binary.ByteOrder
		expected wkb.Point
	}{
		{
			bytes: []byte{70, 129, 246, 35, 46, 74, 93, 192, 3, 70, 27, 60, 175, 91, 64, 64},
			bom:   binary.LittleEndian,
			expected: wkb.Point{
				X: -117.15906619141342,
				Y: 32.71628524142945,
			},
		},
	}

	var p wkb.Point
	for i, test := range testcases {
		buf := bytes.NewReader(test.bytes)
		p.Decode(test.bom, buf)
		if p.X != test.expected.X {
			t.Errorf("Failed Test %v: X value of point is bad. Expected %v, Got %v\n", i, test.expected.X, p.X)
		}
		if p.Y != test.expected.Y {
			t.Errorf("Failed Test %v: Y value of point is bad. Expected %v, Got %v\n", i, test.expected.Y, p.Y)
		}
	}
}
