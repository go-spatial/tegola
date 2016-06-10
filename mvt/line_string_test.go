package mvt_test

import (
	"bytes"
	"testing"

	"github.com/terrando/tegola/mvt"
)

func TestMarshalLinestring(t *testing.T) {
	testcases := []struct {
		l1       mvt.Linestring
		p2       mvt.Point
		expected []byte
	}{
		{
			l1: mvt.Linestring{
				mvt.Point{
					X: 2,
					Y: 2,
				},
				mvt.Point{
					X: 2,
					Y: 10,
				},
				mvt.Point{
					X: 10,
					Y: 10,
				},
			},
			p2:       mvt.Point{0, 0},
			expected: []byte{9, 4, 4, 18, 0, 16, 16, 0},
		},
	}

	for i, test := range testcases {
		result := test.l1.Marshal(test.p2)
		if !bytes.Equal(result, test.expected) {
			t.Errorf("Failed Test %v: Expected %v, Got %v\n", i, test.expected, result)
		}
	}
}
