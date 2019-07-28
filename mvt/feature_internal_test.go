package mvt

import (
	"context"
	"testing"

	"github.com/go-spatial/tegola/basic"
	vectorTile "github.com/go-spatial/tegola/mvt/vector_tile"
)

func TestEncodeGeometry(t *testing.T) {
	type tcase struct {
		geo          basic.Geometry
		geomType     vectorTile.Tile_GeomType
		expectedGeom []uint32
		expectedErr  error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			g, gtype, err := encodeGeometry(context.Background(), tc.geo)
			if tc.expectedErr != err {
				t.Errorf("error, expected %v got %v", tc.expectedErr, err)
			}

			if gtype != tc.geomType {
				t.Errorf("geometry type, expected %v got %v", tc.geomType, gtype)
			}

			if len(g) != len(tc.expectedGeom) {
				t.Errorf("geometry length, expected %v got %v ", len(tc.expectedGeom), len(g))
				t.Logf("geometries, expected %v got %v", tc.expectedGeom, g)
			}

			for j := range tc.expectedGeom {
				if j < len(g) && tc.expectedGeom[j] != g[j] {
					t.Errorf("geometry at %v, expected %v got %v", j, tc.expectedGeom[j], g[j])
					t.Logf("geometry, expected %v got %v", tc.expectedGeom, g)
					break
				}
			}
		}
	}

	tests := map[string]tcase{
		"nil geo": tcase{
			geo:          nil,
			geomType:     vectorTile.Tile_UNKNOWN,
			expectedGeom: []uint32{},
			expectedErr:  ErrNilGeometryType,
		},
		"point 1": tcase{
			geo:          basic.Point{1, 1},
			geomType:     vectorTile.Tile_POINT,
			expectedGeom: []uint32{9, 2, 2},
		},
		"point 2": tcase{
			geo:          basic.Point{25, 17},
			geomType:     vectorTile.Tile_POINT,
			expectedGeom: []uint32{9, 50, 34},
		},
		"multi point": tcase{
			geo: basic.MultiPoint{
				basic.Point{5, 7},
				basic.Point{3, 2},
			},
			geomType:     vectorTile.Tile_POINT,
			expectedGeom: []uint32{17, 10, 14, 3, 9},
		},
		"linestring": tcase{
			geo: basic.Line{
				basic.Point{2, 2},
				basic.Point{2, 10},
				basic.Point{10, 10},
			},
			geomType:     vectorTile.Tile_LINESTRING,
			expectedGeom: []uint32{9, 4, 4, 18, 0, 16, 16, 0},
		},
		"multi linestring": tcase{
			geo: basic.MultiLine{
				basic.Line{
					basic.Point{2, 2},
					basic.Point{2, 10},
					basic.Point{10, 10},
				},
				basic.Line{
					basic.Point{1, 1},
					basic.Point{3, 5},
				},
			},
			geomType:     vectorTile.Tile_LINESTRING,
			expectedGeom: []uint32{9, 4, 4, 18, 0, 16, 16, 0, 9, 17, 17, 10, 4, 8},
		},
		"polygon": tcase{
			geo: basic.Polygon{
				basic.Line{
					basic.Point{3, 6},
					basic.Point{8, 12},
					basic.Point{20, 34},
				},
			},
			geomType:     vectorTile.Tile_POLYGON,
			expectedGeom: []uint32{9, 6, 12, 18, 10, 12, 24, 44, 15},
		},
		"multi polygon": tcase{
			geo: basic.MultiPolygon{
				basic.Polygon{
					basic.Line{
						basic.Point{0, 0},
						basic.Point{10, 0},
						basic.Point{10, 10},
						basic.Point{0, 10},
					},
				},
				basic.Polygon{
					basic.Line{
						basic.Point{11, 11},
						basic.Point{20, 11},
						basic.Point{20, 20},
						basic.Point{11, 20},
					},
					basic.Line{
						basic.Point{13, 13},
						basic.Point{13, 17},
						basic.Point{17, 17},
						basic.Point{17, 13},
					},
				},
			},
			geomType:     vectorTile.Tile_POLYGON,
			expectedGeom: []uint32{9, 0, 0, 26, 20, 0, 0, 20, 19, 0, 15, 9, 22, 2, 26, 18, 0, 0, 18, 17, 0, 15, 9, 4, 13, 26, 0, 8, 8, 0, 0, 7, 15},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
