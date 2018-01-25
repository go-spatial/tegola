package mvt

import (
	"fmt"
	"testing"

	"context"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

func TestEncodeGeometry(t *testing.T) {
	type tc struct {
		desc string `tbltest:"desc"`
		geo  basic.Geometry
		typ  vectorTile.Tile_GeomType
		bbox tegola.BoundingBox
		egeo []uint32
		eerr error
	}
	tile := tegola.NewTile(20, 0, 0)
	fromPixel := func(x, y float64) *basic.Point {
		pt, err := tile.FromPixel(tegola.WebMercator, [2]float64{x, y})
		if err != nil {
			panic(fmt.Sprintf("error trying to convert %v,%v to WebMercator. %v", x, y, err))
		}
		bpt := basic.Point(pt)
		return &bpt
	}
	fn := func(i int, tcase tc) {
		g, gtype, err := encodeGeometry(context.Background(), tcase.geo, tile, true)
		if tcase.eerr != err {
			t.Errorf("[%v] error, Expected %v Got %v", i, tcase.eerr, err)
		}
		if gtype != tcase.typ {
			t.Errorf("[%v] geometry type, Expected %v Got %v", i, tcase.typ, gtype)
		}
		if len(g) != len(tcase.egeo) {
			t.Errorf("[%v] geometry length, Expected %v Got %v ", i, len(tcase.egeo), len(g))
			t.Logf("[%v] Geometries, Expected %v Got %v", i, tcase.egeo, g)
		}
		for j := range tcase.egeo {

			if j < len(g) && tcase.egeo[j] != g[j] {
				t.Errorf("[%v] Geometry at %v, Expected %v Got %v", i, j, tcase.egeo[j], g[j])
				t.Logf("[%v] Geometry, Expected %v Got %v", i, tcase.egeo, g)
				break
			}
		}
	}
	tbltest.Cases(
		tc{ //0
			geo:  nil,
			typ:  vectorTile.Tile_UNKNOWN,
			egeo: []uint32{},
			eerr: ErrNilGeometryType,
		},
		tc{ // 1
			geo:  fromPixel(1, 1),
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{9, 2, 2},
		},
		tc{ // 2
			geo:  fromPixel(25, 16),
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{9, 50, 32},
		},
		tc{ // 3
			geo:  basic.MultiPoint{*fromPixel(5, 7), *fromPixel(3, 2)},
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{17, 10, 14, 3, 11},
		},
		tc{ // 4
			geo:  basic.Line{*fromPixel(2, 2), *fromPixel(2, 10), *fromPixel(10, 10)},
			typ:  vectorTile.Tile_LINESTRING,
			egeo: []uint32{9, 2, 2, 18, 0, 16, 16, 0},
		},
		/*
				Disabling this for now; Currently it's getting clipped out because the transformation. scaling and clipping are intermingled with the encoding
				process. Once that is seperated out we can have a better encoding test. Issue #224
			tc{ // 5
				geo: basic.MultiLine{
					basic.Line{*fromPixel(2, 2), *fromPixel(2, 10), *fromPixel(10, 10)},
					basic.Line{*fromPixel(1, 1), *fromPixel(3, 5)},
				},
				typ:  vectorTile.Tile_LINESTRING,
				egeo: []uint32{9, 2, 2, 18, 0, 16, 16, 0, 9, 15, 15, 10, 4, 8},
			},
				tc{ // 6
					geo: basic.Polygon{
						basic.Line{
							*fromPixel(3, 6),
							*fromPixel(8, 12),
							*fromPixel(20, 34),
						},
					},
					typ: vectorTile.Tile_POLYGON,
					egeo: []uint32{9, 6, 12, 26, 10, 12, 24, 44, 23, 39, 15},
				},
		*/
		tc{ // 7
			geo: basic.MultiPolygon{
				basic.Polygon{ // basic.Polygon len(000002).
					basic.Line{ // basic.Line len(000004) direction(clockwise) line(00).
						*fromPixel(0, 0),
						*fromPixel(10, 0),
						*fromPixel(10, 10),
						*fromPixel(0, 10),
					},
				},
				basic.Polygon{ // basic.Polygon len(000002).
					basic.Line{ // basic.Line len(000004) direction(clockwise) line(00).
						*fromPixel(11, 11),
						*fromPixel(20, 11),
						*fromPixel(20, 20),
						*fromPixel(11, 20),
					},
					basic.Line{ // basic.Line len(000004) direction(counter clockwise) line(01).
						*fromPixel(13, 13),
						*fromPixel(13, 17),
						*fromPixel(17, 17),
						*fromPixel(17, 13),
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
			egeo: []uint32{9, 0, 0, 26, 18, 0, 0, 18, 17, 0, 15, 9, 22, 4, 26, 18, 0, 0, 18, 17, 0, 15, 9, 2, 15, 26, 0, 8, 8, 0, 0, 7, 15},
		},
	).Run(fn)
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
		// TODO test to make sure we got the correct feature

	}
}
