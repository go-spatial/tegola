package geojson_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding"
	"github.com/terranodo/tegola/geom/encoding/geojson"
)

func TestFeatureMarshalJSON(t *testing.T) {
	type tcase struct {
		geom        geom.Geometry
		expected    []byte
		expectedErr json.MarshalerError
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		f := geojson.Feature{
			Geometry: geojson.Geometry{tc.geom},
		}

		output, err := json.Marshal(f)
		if err != nil && err.Error() != tc.expectedErr.Error() {
			t.Errorf("expected err %v got %v", tc.expectedErr.Error(), err)
			return
		}

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("expected %v got %v", string(tc.expected), string(output))
			return
		}
	}

	tests := map[string]tcase{
		"point": {
			geom:     geom.Point{12.2, 17.7},
			expected: []byte(`{"type":"Feature","geometry":{"type":"Point","coordinates":[12.2,17.7]},"properties":null}`),
		},
		"multi point": {
			geom:     geom.MultiPoint{{12.2, 17.7}, {13.3, 18.8}},
			expected: []byte(`{"type":"Feature","geometry":{"type":"MultiPoint","coordinates":[[12.2,17.7],[13.3,18.8]]},"properties":null}`),
		},
		"linestring": {
			geom:     geom.LineString{geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}},
			expected: []byte(`{"type":"Feature","geometry":{"type":"LineString","coordinates":[[3.2,4.3],[5.4,6.5],[7.6,8.7],[9.8,10.9]]},"properties":null}`),
		},
		"multi linestring": {
			geom: geom.MultiLineString{
				{geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9}},
				{geom.Point{2.3, 3.4}, geom.Point{4.5, 5.6}, geom.Point{6.7, 7.8}, geom.Point{8.9, 9.10}},
				{geom.Point{2.2, 3.3}, geom.Point{4.4, 5.5}, geom.Point{6.6, 7.7}, geom.Point{8.8, 9.9}},
			},
			expected: []byte(`{"type":"Feature","geometry":{"type":"MultiLineString","coordinates":[[[3.2,4.3],[5.4,6.5],[7.6,8.7],[9.8,10.9]],[[2.3,3.4],[4.5,5.6],[6.7,7.8],[8.9,9.1]],[[2.2,3.3],[4.4,5.5],[6.6,7.7],[8.8,9.9]]]},"properties":null}`),
		},
		"polygon": {
			geom: geom.Polygon{
				{
					geom.Point{3.2, 4.3}, geom.Point{5.4, 6.5}, geom.Point{7.6, 8.7}, geom.Point{9.8, 10.9},
				},
			},
			expected: []byte(`{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[3.2,4.3],[5.4,6.5],[7.6,8.7],[9.8,10.9],[3.2,4.3]]]},"properties":null}`),
		},
		"multi polygon": {
			geom: geom.MultiPolygon{
				// Polygon 1 w/ holes
				geom.Polygon{
					// Outer ring
					{
						geom.Point{10.1, 10.1},
						geom.Point{5.5, 20.2},
						geom.Point{7.7, 30.3},
						geom.Point{30.3, 30.3},
						geom.Point{30.3, 10.1},
						geom.Point{10.1, 10.1},
					},
					// Hole 1
					{
						geom.Point{15.5, 15.5}, geom.Point{11.1, 14.4}, geom.Point{11.1, 11.1}, geom.Point{15.5, 11.1},
					},
					// Hole 2
					{
						geom.Point{25.5, 25.5}, geom.Point{21.1, 24.4}, geom.Point{21.1, 21.1}, geom.Point{25.5, 21.1},
					},
				},
				// Polygon 2, simple
				geom.Polygon{
					// Hole 2
					{
						geom.Point{75.5, 75.5}, geom.Point{71.1, 74.4}, geom.Point{71.1, 71.1}, geom.Point{75.5, 71.1},
					},
				},
			},
			expected: []byte(`{"type":"Feature","geometry":{"type":"MultiPolygon","coordinates":[[[[10.1,10.1],[5.5,20.2],[7.7,30.3],[30.3,30.3],[30.3,10.1],[10.1,10.1]],[[15.5,15.5],[11.1,14.4],[11.1,11.1],[15.5,11.1],[15.5,15.5]],[[25.5,25.5],[21.1,24.4],[21.1,21.1],[25.5,21.1],[25.5,25.5]]],[[[75.5,75.5],[71.1,74.4],[71.1,71.1],[75.5,71.1],[75.5,75.5]]]]},"properties":null}`),
		},
		"geometry collection": {
			geom: geom.Collection{
				geom.Point{12.2, 17.7},
				geom.MultiPoint{{12.2, 17.7}, {13.3, 18.8}},
				geom.LineString{{3.2, 4.3}, {5.4, 6.5}, {7.6, 8.7}, {9.8, 10.9}},
			},
			expected: []byte(`{"type":"Feature","geometry":{"type":"GeometryCollection","geometries":[{"type":"Point","coordinates":[12.2,17.7]},{"type":"MultiPoint","coordinates":[[12.2,17.7],[13.3,18.8]]},{"type":"LineString","coordinates":[[3.2,4.3],[5.4,6.5],[7.6,8.7],[9.8,10.9]]}]},"properties":null}`),
		},
		"nil geom": {
			geom: nil,
			expectedErr: json.MarshalerError{
				Type: reflect.TypeOf(geojson.Geometry{}),
				Err:  encoding.ErrUnknownGeometry{nil},
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
