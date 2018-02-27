package geom_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom"
)

func TestPolygonSetter(t *testing.T) {
	testcases := []struct {
		points   [][][2]float64
		setter   geom.PolygonSetter
		expected geom.PolygonSetter
	}{
		{
			points: [][][2]float64{
				{
					{10, 20},
					{30, 40},
					{-10, -5},
					{10, 20},
				},
			},
			setter: &geom.Polygon{
				{
					{15, 20},
					{35, 40},
					{-15, -5},
					{25, 20},
				},
			},
			expected: &geom.Polygon{
				{
					{10, 20},
					{30, 40},
					{-10, -5},
					{10, 20},
				},
			},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetLinearRings(tc.points)
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
