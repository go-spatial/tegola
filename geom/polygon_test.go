package geom_test

import (
	"testing"

	"github.com/terrando/tegola/geom"
)

func TestPolygonArea(t *testing.T) {
	testcases := []struct {
		poly     geom.Polygon
		expected float64
	}{
		{
			poly: geom.Polygon{
				Points: []geom.Point{
					geom.Point{
						X: 3.0,
						Y: 4.0,
					},
					geom.Point{
						X: 5.0,
						Y: 11.0,
					},
					geom.Point{
						X: 12.0,
						Y: 8.0,
					},
					geom.Point{
						X: 9.0,
						Y: 5.0,
					},
					geom.Point{
						X: 5.0,
						Y: 6.0,
					},
				},
			},
			expected: 30,
		},
	}

	for i, test := range testcases {
		if test.expected != test.poly.Area() {
			t.Errorf("Failed Test %v: Expected %v, Got %v\n", i, test.expected, test.poly.Area())
		}
	}
}
