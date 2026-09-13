package atlas

import (
	"reflect"
	"testing"

	"github.com/go-spatial/geom"
)

// TestFlattenGeometryCollections verifies that geom.Collection values (as
// produced by GPKG GEOMETRYCOLLECTION rows) are recursively expanded into
// their leaf geometries, since the MVT/vector tile spec has no "collection"
// feature type and can only encode Point/LineString/Polygon (and Multi
// variants) as a single feature.
func TestFlattenGeometryCollections(t *testing.T) {
	tests := map[string]struct {
		geo      geom.Geometry
		expected []geom.Geometry
	}{
		"non collection passthrough": {
			geo:      geom.Point{1, 2},
			expected: []geom.Geometry{geom.Point{1, 2}},
		},
		"empty collection": {
			geo:      geom.Collection{},
			expected: []geom.Geometry{},
		},
		"flat collection": {
			geo: geom.Collection{
				geom.Point{1, 2},
				geom.LineString{{1, 2}, {3, 4}},
			},
			expected: []geom.Geometry{
				geom.Point{1, 2},
				geom.LineString{{1, 2}, {3, 4}},
			},
		},
		"nested collection": {
			geo: geom.Collection{
				geom.Point{1, 2},
				geom.Collection{
					geom.LineString{{1, 2}, {3, 4}},
					geom.Collection{
						geom.Point{5, 6},
					},
				},
			},
			expected: []geom.Geometry{
				geom.Point{1, 2},
				geom.LineString{{1, 2}, {3, 4}},
				geom.Point{5, 6},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := flattenGeometryCollections(tc.geo)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("flattenGeometryCollections() = %#v, expected %#v", got, tc.expected)
			}
		})
	}
}
