package geom_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom"
)

func TestMultiLineStringSetter(t *testing.T) {
	testcases := []struct {
		points   [][][2]float64
		setter   geom.MultiLineStringSetter
		expected geom.MultiLineStringSetter
	}{
		{
			points: [][][2]float64{
				{
					{15, 20},
					{35, 40},
				},
				{
					{-15, -5},
					{20, 20},
				},
			},
			setter: &geom.MultiLineString{
				{
					{10, 20},
					{30, 40},
				},
				{
					{-10, -5},
					{15, 20},
				},
			},
			expected: &geom.MultiLineString{
				{
					{15, 20},
					{35, 40},
				},
				{
					{-15, -5},
					{20, 20},
				},
			},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetLineStrings(tc.points)
		if err != nil {
			t.Errorf("test case (%v) failed. err: %v", i, err)
			continue
		}

		// compare the results
		if !reflect.DeepEqual(tc.expected, tc.setter) {
			t.Errorf("test case (%v) failed. Expected (%v) does not match result (%v)", i, tc.expected, tc.setter)
		}
	}
}
