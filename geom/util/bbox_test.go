package util_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/geom/util"
)

func TestBBox(t *testing.T) {
	testcases := []struct {
		points   [][2]float64
		expected util.BoundingBox
	}{

		{
			points: [][2]float64{
				{1.0, 2.0},
			},
			expected: util.BoundingBox{
				[2]float64{1.0, 2.0},
				[2]float64{1.0, 2.0},
			},
		},
		{
			points: [][2]float64{
				{0.0, 0.0},
				{6.0, 4.0},
				{3.0, 7.0},
			},
			expected: util.BoundingBox{
				[2]float64{0.0, 0.0},
				[2]float64{6.0, 7.0},
			},
		},
	}

	for i, tc := range testcases {
		output := util.BBox(tc.points...)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("test case (%v) failed. output (%+v) does not match expected (%+v)", i, output, tc.expected)
		}
	}
}
