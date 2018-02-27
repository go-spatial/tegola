package geom_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom"
)

func TestMultiPolygonSetter(t *testing.T) {
	testcases := []struct {
		points   [][][][2]float64
		setter   geom.MultiPolygonSetter
		expected geom.MultiPolygonSetter
	}{
		{
			points: [][][][2]float64{
				{
					{
						{10, 20},
						{30, 40},
						{-10, -5},
						{10, 20},
					},
				},
			},
			setter: &geom.MultiPolygon{
				{
					{
						{15, 20},
						{30, 45},
						{-15, -5},
						{10, 25},
					},
				},
			},
			expected: &geom.MultiPolygon{
				{
					{
						{10, 20},
						{30, 40},
						{-10, -5},
						{10, 20},
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetPolygons(tc.points)
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
