package geom_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/geom"
)

func TestMultiPointSetter(t *testing.T) {
	testcases := []struct {
		points   [][2]float64
		setter   geom.MultiPointSetter
		expected geom.MultiPointSetter
	}{
		{
			points: [][2]float64{
				{15, 20},
				{35, 40},
				{-15, -5},
			},
			setter: &geom.MultiPoint{
				{10, 20},
				{30, 40},
				{-10, -5},
			},
			expected: &geom.MultiPoint{
				{15, 20},
				{35, 40},
				{-15, -5},
			},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetPoints(tc.points)
		if err != nil {
			t.Errorf("test case (%v) failed. err: %v", i, err)
			continue
		}

		//	compare the results
		if !reflect.DeepEqual(tc.expected, tc.setter) {
			t.Errorf("test case (%v) failed. Expected (%v) does not match result (%v)", i, tc.expected, tc.setter)
		}
	}
}
