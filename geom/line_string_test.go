package geom_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/geom"
)

func TestLineStringSetter(t *testing.T) {
	testcases := []struct {
		points   [][2]float64
		setter   geom.LineStringSetter
		expected geom.LineStringSetter
	}{
		{
			points: [][2]float64{
				{15, 20},
				{35, 40},
				{-15, -5},
			},
			setter: &geom.LineString{
				{10, 20},
				{30, 40},
				{-10, -5},
			},
			expected: &geom.LineString{
				{15, 20},
				{35, 40},
				{-15, -5},
			},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetVerticies(tc.points)
		if err != nil {
			t.Errorf("[%v] set verticies, expected nil got %v", i, err)
			continue
		}

		//	compare the results
		if !reflect.DeepEqual(tc.expected, tc.setter) {
			t.Errorf("[%v] set verticies, expected %v got %v", i, tc.expected, tc.setter)
		}
	}
}
