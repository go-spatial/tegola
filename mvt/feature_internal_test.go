package mvt

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/mvt/vector_tile"
)

func TestScaleLinestring(t *testing.T) {
	tile := tegola.NewTile(20, 0, 0)
	cursor := NewCursor(tile)

	newLine := func(ptpairs ...float64) (ln basic.Line) {
		for i, j := 0, 1; j < len(ptpairs); i, j = i+2, j+2 {
			pt, err := tile.FromPixel(tegola.WebMercator, [2]float64{ptpairs[i], ptpairs[j]})
			if err != nil {
				panic(fmt.Sprintf("error trying to convert %v,%v to WebMercator. %v", ptpairs[i], ptpairs[j], err))
			}

			ln = append(ln, basic.Point(pt))
		}

		return ln
	}

	type tcase struct {
		g tegola.LineString
		e basic.Line
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			got := cursor.scalelinestr(tc.g)

			if !reflect.DeepEqual(tc.e, got) {
				t.Errorf("expected %v got %v", tc.e, got)
			}
		}
	}

	tests := map[string]tcase{
		"duplicate pt simple line": {
			g: basic.NewLine(9.0, 9.0, 9.0, 9.0),
		},
		"simple line": {
			g: newLine(10.0, 10.0, 11.0, 11.0),
			e: basic.NewLine(9.0, 9.0, 11.0, 11.0),
		},
		"simple line 3pt": {
			g: newLine(10.0, 10.0, 11.0, 10.0, 11.0, 15.0),
			e: basic.NewLine(9.0, 9.0, 11.0, 9.0, 11.0, 14.0),
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestEncodeGeometry(t *testing.T) {
	type tcase struct {
		desc string
		geo  basic.Geometry
		typ  vectorTile.Tile_GeomType
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

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			g, gtype, err := encodeGeometry(context.Background(), tc.geo, tile, true)
			if tc.eerr != err {
				t.Errorf("error, Expected %v Got %v", tc.eerr, err)
			}
			if gtype != tc.typ {
				t.Errorf("geometry type, Expected %v Got %v", tc.typ, gtype)
			}
			if len(g) != len(tc.egeo) {
				t.Errorf("geometry length, Expected %v Got %v ", len(tc.egeo), len(g))
				t.Logf("Geometries, Expected %v Got %v", tc.egeo, g)
			}
			for j := range tc.egeo {

				if j < len(g) && tc.egeo[j] != g[j] {
					t.Errorf("Geometry at %v, Expected %v Got %v", j, tc.egeo[j], g[j])
					t.Logf("Geometry, Expected %v Got %v", tc.egeo, g)
					break
				}
			}
		}
	}

	tests := map[string]tcase{
		"0": tcase{
			geo:  nil,
			typ:  vectorTile.Tile_UNKNOWN,
			egeo: []uint32{},
			eerr: ErrNilGeometryType,
		},
		"1": tcase{
			geo:  fromPixel(1, 1),
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{9, 2, 2},
		},
		"2": tcase{
			geo:  fromPixel(25, 16),
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{9, 50, 32},
		},
		"3": tcase{
			geo:  basic.MultiPoint{*fromPixel(5, 7), *fromPixel(3, 2)},
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{17, 10, 14, 3, 11},
		},
		"4": tcase{
			geo:  basic.Line{*fromPixel(2, 2), *fromPixel(2, 10), *fromPixel(10, 10)},
			typ:  vectorTile.Tile_LINESTRING,
			egeo: []uint32{9, 2, 2, 18, 0, 16, 16, 0},
		},
		/*
				Disabling this for now; Currently it's getting clipped out because the transformation. scaling and clipping are intermingled with the encoding
				process. Once that is separated out we can have a better encoding test. Issue #224
			"5":tcase{ // 5
				geo: basic.MultiLine{
					basic.Line{*fromPixel(2, 2), *fromPixel(2, 10), *fromPixel(10, 10)},
					basic.Line{*fromPixel(1, 1), *fromPixel(3, 5)},
				},
				typ:  vectorTile.Tile_LINESTRING,
				egeo: []uint32{9, 2, 2, 18, 0, 16, 16, 0, 9, 15, 15, 10, 4, 8},
			},
			"6":tcase{ // 6
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
		"7": tcase{
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
			typ:  vectorTile.Tile_POLYGON,
			egeo: []uint32{9, 0, 0, 26, 18, 0, 0, 18, 17, 0, 15, 9, 22, 4, 26, 18, 0, 0, 18, 17, 0, 15, 9, 2, 15, 26, 0, 8, 8, 0, 0, 7, 15},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
