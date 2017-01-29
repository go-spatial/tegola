package region

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
)

func TestAxis(t *testing.T) {
	type testcase struct {
		line          maths.Line
		doesIntersect [4]bool
		pt            [4]maths.Pt
	}

	r := New(maths.CounterClockwise, maths.Pt{5, 2}, maths.Pt{11, 9})

	test := tbltest.Cases(
		testcase{
			line:          maths.Line{maths.Pt{-3, 1}, maths.Pt{-3, 10}},
			doesIntersect: [4]bool{false, false, false, false},
		},
	)
	test.Run(func(idx int, tc testcase) {
		for a, i := r.FirstAxis(), 0; a != nil; a, i = a.Next(), i+1 {
			pt, ok := a.Intersect(tc.line)
			if ok != tc.doesIntersect[i] {
				t.Errorf("Test(%v) Does Intersect is not correct got %v [[%v]] want %v", idx, ok, pt, tc.doesIntersect[i])
			}
			if tc.doesIntersect[i] && !tc.pt[i].IsEqual(pt) {
				t.Errorf("Test(%v) Point is not correct got %v want %v", idx, pt, tc.pt)
			}
		}
	})
}
