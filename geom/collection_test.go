package geom_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/geom"
)

func TestCollectionSetter(t *testing.T) {
	testcases := []struct {
		geoms    []geom.Geometry
		setter   geom.CollectionSetter
		expected geom.CollectionSetter
	}{
		{
			geoms: []geom.Geometry{
				&geom.Point{10, 20},
				&geom.LineString{
					{30, 40},
					{50, 60},
				},
			},
			setter: &geom.Collection{
				&geom.Point{15, 25},
				&geom.MultiPoint{
					{35, 45},
					{55, 65},
				},
			},
			expected: &geom.Collection{
				&geom.Point{10, 20},
				&geom.LineString{
					{30, 40},
					{50, 60},
				},
			},
		},
	}

	for i, tc := range testcases {
		err := tc.setter.SetGeometries(tc.geoms)
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
