package plyg

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/internal/assert"
)

func TestGetTrianglesForCol(t *testing.T) {
	type tcase struct {
		Pt2Maxy    map[maths.Pt]int64
		Col1, Col2 []maths.Pt
		Tris       []tri
		Err        error
	}
	fn := func(tc tcase) func(*testing.T) {
		ctx := context.Background()
		return func(t *testing.T) {
			tris, err := _getTrianglesForCol(ctx, tc.Pt2Maxy, tc.Col1, tc.Col2)
			if equ := assert.ErrorEquality(tc.Err, err); !equ.IsEqual {
				t.Errorf("%v", equ)
				return
			}
			if !reflect.DeepEqual(tc.Tris, tris) {
				t.Errorf("incorrect triangles, Expected %v Got %v", tc.Tris, tris)
				if len(tc.Tris) != len(tris) {
					t.Logf("length is not the same, Expected %v Got %v", len(tc.Tris), len(tris))
				}
				for i := range tc.Tris {
					if i < len(tris) {
						t.Logf("triangle %v,\n\tExpected %v\n\tGot      %v", i, tc.Tris[i], tris[i])
					} else {
						t.Logf("triangle %v,\n\tExpected %v\n\tGot      nil ", i, tc.Tris[i])
					}
				}
				if len(tris) > len(tc.Tris) {
					i := len(tc.Tris)
					for ; i < len(tris); i++ {
						t.Logf("triangle %v,\n\tExpected nil\n\tGot      %v ", i, tris[i])
					}
				}
			}
		}
	}
	tests := map[string]tcase{
		"simple": {
			Col1: []maths.Pt{
				{0, 0},
				{0, 1},
			},
			Col2: []maths.Pt{
				{1, 0},
				{1, 1},
			},
			Tris: []tri{
				{0, 2, 0, 1},
				{1, 1, 0, 2},
			},
		},
		"simplel1": {
			Col1: []maths.Pt{{0, 1}},
			Col2: []maths.Pt{{1, 0}, {1, 1}},
			Tris: []tri{{0, 1, 0, 2}},
		},
		"simpler1": {
			Col1: []maths.Pt{{0, 0}, {0, 1}},
			Col2: []maths.Pt{{1, 1}},
			Tris: []tri{{0, 2, 0, 1}},
		},
		"with_maxy1": {
			Pt2Maxy: map[maths.Pt]int64{
				{0, 1}: 300,
			},
			Col1: []maths.Pt{
				{0, 1},
				{0, 2},
				{0, 3},
			},
			Col2: []maths.Pt{
				{1, 1},
				{1, 2},
				{1, 3},
			},
			Tris: []tri{

				{0, 1, 0, 2},
				{0, 1, 1, 2},
				{0, 2, 2, 1},
				{1, 2, 2, 1},
			},
		},
		"with maxy": {
			Pt2Maxy: map[maths.Pt]int64{
				{0, 1}: 300,
			},
			Col1: []maths.Pt{
				{0, 0},
				{0, 1},
				{0, 2},
				{0, 3},
				{0, 4},
			},
			Col2: []maths.Pt{
				{1, 0},
				{1, 1},
				{1, 2},
				{1, 3},
				{1, 4},
			},
			Tris: []tri{
				{0, 2, 0, 1},
				{1, 1, 0, 2},

				{1, 1, 1, 2},
				{1, 1, 2, 2},
				{1, 2, 3, 1},
				{2, 2, 3, 1},

				{3, 2, 3, 1},
				{4, 1, 3, 2},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}
