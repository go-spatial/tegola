package mvt

import (
	"testing"

	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

func TestEncodeGeometry(t *testing.T) {
	/*
		complexGemo := basic.Polygon{
			basic.Line{
				basic.Point{8, 8.5},
				basic.Point{9, 9},
				basic.Point{20, 20},
				basic.Point{11, 20},
			},
		}
	*/
	testcases := []struct {
		desc string `"test":"desc"`
		geo  basic.Geometry
		typ  vectorTile.Tile_GeomType
		bbox tegola.BoundingBox
		egeo []uint32
		eerr error
	}{
		{ //0
			geo: nil,
			typ: vectorTile.Tile_UNKNOWN,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{},
			eerr: ErrNilGeometryType,
		},
		{ // 1
			geo: basic.Point{1, 1},
			typ: vectorTile.Tile_POINT,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{9, 2, 2},
		},
		{ // 2
			geo: basic.Point{25, 17},
			typ: vectorTile.Tile_POINT,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{9, 50, 34},
		},
		{ // 3
			geo: basic.MultiPoint{basic.Point{5, 7}, basic.Point{3, 2}},
			typ: vectorTile.Tile_POINT,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{17, 10, 14, 3, 9},
		},
		{ // 4
			geo: basic.Line{basic.Point{2, 2}, basic.Point{2, 10}, basic.Point{10, 10}},
			typ: vectorTile.Tile_LINESTRING,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{9, 4, 4, 18, 0, 16, 16, 0},
		},
		{ // 5
			geo: basic.MultiLine{
				basic.Line{basic.Point{2, 2}, basic.Point{2, 10}, basic.Point{10, 10}},
				basic.Line{basic.Point{1, 1}, basic.Point{3, 5}},
			},
			typ: vectorTile.Tile_LINESTRING,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{9, 4, 4, 18, 0, 16, 16, 0, 9, 17, 17, 10, 4, 8},
		},
		{ // 6
			geo: basic.Polygon{
				basic.Line{
					basic.Point{3, 6},
					basic.Point{8, 12},
					basic.Point{20, 34},
				},
			},
			typ: vectorTile.Tile_POLYGON,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{9, 6, 12, 26, 10, 12, 24, 44, 23, 39, 15},
		},
		{ // 7
			geo: basic.MultiPolygon{
				basic.Polygon{ // basic.Polygon len(000002).
					basic.Line{ // basic.Line len(000004) direction(clockwise) line(00).
						basic.Point{0, 0},
						basic.Point{10, 0},
						basic.Point{10, 10},
						basic.Point{0, 10},
					},
				},
				basic.Polygon{ // basic.Polygon len(000002).
					basic.Line{ // basic.Line len(000004) direction(clockwise) line(00).
						basic.Point{11, 11},
						basic.Point{20, 11},
						basic.Point{20, 20},
						basic.Point{11, 20},
					},
					basic.Line{ // basic.Line len(000004) direction(counter clockwise) line(01).
						basic.Point{13, 13},
						basic.Point{13, 17},
						basic.Point{17, 17},
						basic.Point{17, 13},
					},
				},
			},
			typ: vectorTile.Tile_POLYGON,
			bbox: tegola.BoundingBox{
				Minx: 0,
				Miny: 0,
				Maxx: 4096,
				Maxy: 4096,
			},
			egeo: []uint32{9, 0, 0, 26, 20, 0, 0, 20, 19, 0, 15, 9, 22, 2, 26, 18, 0, 0, 18, 17, 0, 15, 9, 4, 13, 26, 0, 8, 8, 0, 0, 7, 15},
		},
	}
	for i, tcase := range testcases {

		g, gtype, err := encodeGeometry(context.Background(), tcase.geo, tcase.bbox, 4096, true)
		if tcase.eerr != err {
			t.Errorf("(%v) Expected error (%v) got (%v) instead", i, tcase.eerr, err)
		}
		if gtype != tcase.typ {
			t.Errorf("(%v) Expected Geometry Type to be %v Got: %v", i, tcase.typ, gtype)
		}
		if len(g) != len(tcase.egeo) {
			t.Errorf("(%v) Geometry length is not what was expected([%v] %v) got ([%v] %v)", i, len(tcase.egeo), tcase.egeo, len(g), g)
		}
		for j := range tcase.egeo {

			if j < len(g) && tcase.egeo[j] != g[j] {
				t.Errorf("(%v) Geometry is not what was expected at (%v) (%v) got (%v)", i, j, tcase.egeo, g)
				break
			}
		}
	}
}

func TestNewFeature(t *testing.T) {
	testcases := []struct {
		geo      tegola.Geometry
		tags     map[string]interface{}
		expected []Feature
	}{
		{
			geo:      nil,
			tags:     nil,
			expected: []Feature{},
		},
	}
	for i, tcase := range testcases {
		got := NewFeatures(tcase.geo, tcase.tags)
		if len(tcase.expected) != len(got) {
			t.Errorf("Test %v: Expected to get %v features got %v features.", i, len(tcase.expected), len(got))
			continue
		}
		if len(tcase.expected) <= 0 {
			continue
		}
		// TOOD test to make sure we got the correct feature

	}
}

func TestNormalizePoint(t *testing.T) {
	testcases := []struct {
		point       basic.Point
		bbox        tegola.BoundingBox
		nx, ny      int64
		layerExtent int
	}{
		{
			point: basic.Point{960000, 6002729},
			bbox: tegola.BoundingBox{
				Minx: 958826.08,
				Miny: 5987771.04,
				Maxx: 978393.96,
				Maxy: 6007338.92,
			},
			nx:          245,
			ny:          3131,
			layerExtent: 4096,
		},
	}

	for i, tcase := range testcases {
		//	new cursor
		c := NewCursor(tcase.bbox, tcase.layerExtent)

		nx, ny := c.ScalePoint(&tcase.point)
		if nx != tcase.nx {
			t.Errorf("Test %v: Expected nx value of %v got %v.", i, tcase.nx, nx)
		}
		if ny != tcase.ny {
			t.Errorf("Test %v: Expected ny value of %v got %v.", i, tcase.ny, ny)
		}
		continue
	}
}
