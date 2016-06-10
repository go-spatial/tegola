package mvt_test

import (
	"bytes"
	"testing"

	"github.com/terrando/tegola/mvt"
)

func TestMarshalPoint(t *testing.T) {
	testcases := []struct {
		p1       mvt.Point
		p2       mvt.Point
		expected []byte
	}{
		{
			p1: mvt.Point{
				X: 25,
				Y: 17,
			},
			p2: mvt.Point{
				X: 0,
				Y: 0,
			},
			expected: []byte{50, 34},
		},
	}

	for i, test := range testcases {
		result := test.p1.Marshal(test.p2)
		if !bytes.Equal(result, test.expected) {
			t.Errorf("Failed Test %v: Expected %v, Got %v\n", i, test.expected, result)
		}
	}
}

func TestMarshalMultiPoint(t *testing.T) {
	testcases := []struct {
		p1       mvt.Point
		p2       mvt.Point
		expected []byte
	}{
		{
			p1: mvt.Point{
				X: 5,
				Y: 7,
			},
			p2: mvt.Point{
				X: 3,
				Y: 2,
			},
			expected: []byte{10, 14, 3, 9},
		},
	}

	for i, test := range testcases {
		result := append(
			test.p1.Marshal(mvt.Point{0, 0}),
			test.p2.Marshal(test.p1)...,
		)

		if !bytes.Equal(result, test.expected) {
			t.Errorf("Failed Test %v: Expected %v, Got %v\n", i, test.expected, result)
		}
	}
}

func TestOffsetPoint(t *testing.T) {
	testcases := []struct {
		p1       mvt.Point
		p2       mvt.Point
		expected mvt.Point
	}{
		{
			p1: mvt.Point{
				X: 5,
				Y: 7,
			},
			p2: mvt.Point{
				X: 3,
				Y: 2,
			},
			expected: mvt.Point{
				X: -2,
				Y: -5,
			},
		},
		{
			p1: mvt.Point{
				X: 2,
				Y: 2,
			},
			p2: mvt.Point{
				X: 2,
				Y: 10,
			},
			expected: mvt.Point{
				X: 0,
				Y: 8,
			},
		},
	}

	for i, test := range testcases {
		result := test.p2.Offset(test.p1)
		if test.expected.X != result.X {
			t.Errorf("Failed Test %v: X value of point is bad. Expected %v, Got %v\n", i, test.expected.X, result.X)
		}
		if test.expected.Y != result.Y {
			t.Errorf("Failed Test %v: Y value of point is bad. Expected %v, Got %v\n", i, test.expected.Y, result.Y)
		}
	}
}
