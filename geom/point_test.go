package geom_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom"
)

func TestPointSetter(t *testing.T) {
	testcases := []struct {
		point    [2]float64
		setter   geom.PointSetter
		expected geom.PointSetter
	}{
		{
			point:    [2]float64{10, 20},
			setter:   &geom.Point{15, 20},
			expected: &geom.Point{10, 20},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetXY(tc.point)
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
