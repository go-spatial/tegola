package geojson

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/geom"
)

func TestClosePolygon(t *testing.T) {
	type tcase struct {
		geom     geom.Polygon
		expected geom.Polygon
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		closePolygon(tc.geom)

		if !reflect.DeepEqual(tc.expected, tc.geom) {
			t.Errorf("expected %v got %v", tc.expected, tc.geom)
			return
		}
	}

	tests := map[string]tcase{
		"needs closing": {
			geom: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9},
				},
			},
			expected: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}, geom.Point{3.2, 4.3},
				},
			},
		},
		"already closed": {
			geom: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}, geom.Point{3.2, 4.3},
				},
			},
			expected: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}, geom.Point{3.2, 4.3},
				},
			},
		},
		"two point only polygon": {
			geom: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5},
				},
			},
			expected: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5},
				},
			},
		},
		"two point polygon 1": {
			geom: geom.Polygon{
				{
					geom.Point{3, 4}, geom.Point{5, 7}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9},
				},
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5},
				},
			},
			expected: geom.Polygon{
				{
					geom.Point{3, 4}, geom.Point{5, 7}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}, geom.Point{3, 4},
				},
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5},
				},
			},
		},
		"two point only polygon 2": {
			geom: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5},
				},
				{
					geom.Point{3, 4}, geom.Point{5, 7}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9},
				},
			},
			expected: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5},
				},
				{
					geom.Point{3, 4}, geom.Point{5, 7}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}, geom.Point{3, 4},
				},
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func BenchmarkClosePolygon(b *testing.B) {
	p := geom.Polygon{
		{
			geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9},
		},
	}

	for n := 0; n < b.N; n++ {
		closePolygon(p)
	}
}
