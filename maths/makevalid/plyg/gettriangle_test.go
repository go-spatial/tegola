package plyg

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/internal/assert"
)

func TestGetTrianglesForCol(t *testing.T) {
	type testcase struct {
		Pt2Maxy    map[maths.Pt]int64
		Col1, Col2 []maths.Pt
		Tris       []tri
		Err        error
	}
	fn := func(tests map[string]testcase) {
		ctx := context.Background()
		for name, tc := range tests {
			tc := tc // make a copy.
			t.Run(name, func(t *testing.T) {
				tris, err := _getTrianglesForCol(ctx, tc.Pt2Maxy, tc.Col1, tc.Col2)
				if equ := assert.ErrorEquality(tc.Err, err); !equ.IsEqual {
					t.Errorf("[%v] %v", name, equ)
					return
				}
				if !reflect.DeepEqual(tc.Tris, tris) {
					t.Errorf("[%v] incorrect triangles, Expected %v Got %v", name, tc.Tris, tris)
					if len(tc.Tris) != len(tris) {
						t.Logf("[%v] length is not the same, Expected %v Got %v", name, len(tc.Tris), len(tris))
					}
					for i := range tc.Tris {
						if i < len(tris) {
							t.Logf("[%v] triangle %v,\n\tExpected %v\n\tGot      %v", name, i, tc.Tris[i], tris[i])
						} else {
							t.Logf("[%v] triangle %v,\n\tExpected %v\n\tGot      nil ", name, i, tc.Tris[i])
						}
					}
					if len(tris) > len(tc.Tris) {
						i := len(tc.Tris)
						for ; i < len(tris); i++ {
							t.Logf("[%v] triangle %v,\n\tExpected nil\n\tGot      %v ", name, i, tris[i])
						}
					}
				}
			})
		}
	}
	fn(map[string]testcase{
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
				maths.Pt{0, 1}: 300,
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
				maths.Pt{0, 1}: 300,
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
	})
}
