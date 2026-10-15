package region

import (
	"testing"

	"github.com/alexeydott/tegola/maths"
	"github.com/gdey/tbltest"
)

func TestAxis_Intersect(t *testing.T) {
	type testcase struct {
		line          maths.Line
		doesIntersect [4]bool
		pt            [4]maths.Pt
	}

	r := New(maths.Clockwise, maths.Pt{X: 0, Y: 0}, maths.Pt{X: 100, Y: 100})

	test := tbltest.Cases(
		testcase{ // 0
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 50, Y: 0}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 1
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 100, Y: 0}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 2
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 150, Y: 0}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 3
			line:          maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: 100, Y: 0}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 4
			line:          maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: 150, Y: 0}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 5
			line:          maths.Line{maths.Pt{X: 100, Y: 0}, maths.Pt{X: 150, Y: 0}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 6
			line:          maths.Line{maths.Pt{X: 0, Y: 50}, maths.Pt{X: 50, Y: 50}},
			doesIntersect: [4]bool{true, false, false, false},
			pt:            [4]maths.Pt{{X: 0, Y: 50}, {}, {}, {}},
		},
		testcase{ // 7
			line:          maths.Line{maths.Pt{X: 0, Y: 50}, maths.Pt{X: 100, Y: 50}},
			doesIntersect: [4]bool{true, false, true, false},
			pt:            [4]maths.Pt{{X: 0, Y: 50}, {}, {X: 100, Y: 50}, {}},
		},
		testcase{ // 8
			line:          maths.Line{maths.Pt{X: 0, Y: 50}, maths.Pt{X: 150, Y: 50}},
			doesIntersect: [4]bool{true, false, true, false},
			pt:            [4]maths.Pt{{X: 0, Y: 50}, {}, {X: 100, Y: 50}, {}},
		},

		testcase{ // 9
			line:          maths.Line{maths.Pt{X: 50, Y: 50}, maths.Pt{X: 100, Y: 50}},
			doesIntersect: [4]bool{false, false, true, false},
			pt:            [4]maths.Pt{{}, {}, {X: 100, Y: 50}, {}},
		},
		testcase{ // 10
			line:          maths.Line{maths.Pt{X: 50, Y: 50}, maths.Pt{X: 150, Y: 50}},
			doesIntersect: [4]bool{false, false, true, false},
			pt:            [4]maths.Pt{{}, {}, {X: 100, Y: 50}, {}},
		},

		testcase{ // 11
			line:          maths.Line{maths.Pt{X: 100, Y: 50}, maths.Pt{X: 150, Y: 50}},
			doesIntersect: [4]bool{false, false, true, false},
			pt:            [4]maths.Pt{{}, {}, {X: 100, Y: 50}, {}},
		},

		testcase{ // 12
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 50, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 13
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 100, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 14
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 150, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 15
			line:          maths.Line{maths.Pt{X: 50, Y: 100}, maths.Pt{X: 100, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 16
			line:          maths.Line{maths.Pt{X: 100, Y: 100}, maths.Pt{X: 150, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 17
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 50}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 18
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 19
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 150}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 20
			line:          maths.Line{maths.Pt{X: 0, Y: 50}, maths.Pt{X: 0, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 21
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 0, Y: 150}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 22
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 50}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 23
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 24
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 150}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 25
			line:          maths.Line{maths.Pt{X: 0, Y: 50}, maths.Pt{X: 0, Y: 100}},
			doesIntersect: [4]bool{false, false, false, false},
		},
		testcase{ // 26
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 0, Y: 150}},
			doesIntersect: [4]bool{false, false, false, false},
		},

		testcase{ // 27
			line:          maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: 50, Y: 50}},
			doesIntersect: [4]bool{false, true, false, false},
			pt:            [4]maths.Pt{{}, {X: 50, Y: 0}, {}, {}},
		},
		testcase{ // 28
			line:          maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: 50, Y: 100}},
			doesIntersect: [4]bool{false, true, false, true},
			pt:            [4]maths.Pt{{}, {X: 50, Y: 0}, {}, {X: 50, Y: 100}},
		},
		testcase{ // 29
			line:          maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: 50, Y: 150}},
			doesIntersect: [4]bool{false, true, false, true},
			pt:            [4]maths.Pt{{}, {X: 50, Y: 0}, {}, {X: 50, Y: 100}},
		},

		testcase{ // 30
			line:          maths.Line{maths.Pt{X: 50, Y: 50}, maths.Pt{X: 50, Y: 100}},
			doesIntersect: [4]bool{false, false, false, true},
			pt:            [4]maths.Pt{{}, {}, {}, {X: 50, Y: 100}},
		},
		testcase{ // 31
			line:          maths.Line{maths.Pt{X: 50, Y: 100}, maths.Pt{X: 50, Y: 150}},
			doesIntersect: [4]bool{false, false, false, true},
			pt:            [4]maths.Pt{{}, {}, {}, {X: 50, Y: 100}},
		},

		// diagonals
		// top left to bottom right.
		testcase{ // 32
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 50, Y: 50}},
			doesIntersect: [4]bool{true, true, false, false},
			pt:            [4]maths.Pt{{X: 0, Y: 0}, {X: 0, Y: 0}, {}, {}},
		},

		testcase{ // 33
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 100, Y: 100}},
			doesIntersect: [4]bool{true, true, true, true},
			pt:            [4]maths.Pt{{X: 0, Y: 0}, {X: 0, Y: 0}, {X: 100, Y: 100}, {X: 100, Y: 100}},
		},
		testcase{ // 34
			line:          maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 150, Y: 150}},
			doesIntersect: [4]bool{true, true, true, true},
			pt:            [4]maths.Pt{{X: 0, Y: 0}, {X: 0, Y: 0}, {X: 100, Y: 100}, {X: 100, Y: 100}},
		},
		// bottom left to top right.
		testcase{ // 35
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 50, Y: 50}},
			doesIntersect: [4]bool{true, false, false, true},
			pt:            [4]maths.Pt{{X: 0, Y: 100}, {}, {}, {X: 0, Y: 100}},
		},

		testcase{ // 36
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 100, Y: 0}},
			doesIntersect: [4]bool{true, true, true, true},
			pt:            [4]maths.Pt{{X: 0, Y: 100}, {X: 100, Y: 0}, {X: 100, Y: 0}, {X: 0, Y: 100}},
		},
		testcase{ // 37
			line:          maths.Line{maths.Pt{X: 0, Y: 100}, maths.Pt{X: 150, Y: -50}},
			doesIntersect: [4]bool{true, true, true, true},
			pt:            [4]maths.Pt{{X: 0, Y: 100}, {X: 100, Y: 0}, {X: 100, Y: 0}, {X: 0, Y: 100}},
		},
	)
	//test.RunOrder = "32"
	test.Run(func(idx int, tc testcase) {
		for a, i := r.FirstAxis(), 0; a != nil; a, i = a.Next(), i+1 {
			pt, ok := a.Intersect(tc.line)
			if ok != tc.doesIntersect[i] {
				t.Errorf("Test(%v) For Axis(%#v) Does Intersect is not correct got %v [[%v]] want %v", idx, a, ok, pt, tc.doesIntersect[i])
			}
			if tc.doesIntersect[i] && !tc.pt[i].IsEqual(pt) {
				t.Errorf("Test(%v) For Axis(%#v) Point is not correct got %v want %v", idx, a, pt, tc.pt[i])
			}
		}
	})
}

func TestAxis_IsInward(t *testing.T) {
	type testcase struct {
		line    maths.Line
		inward  [4]bool
		err     [4]error
		winding maths.WindingOrder
	}

	test := tbltest.Cases(
		testcase{ // 0
			line:   maths.Line{maths.Pt{X: -50, Y: 0}, maths.Pt{X: 50, Y: 0}},
			inward: [4]bool{true, false, false, false},
			err:    [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
		},
		testcase{ // 1
			line:   maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 50, Y: 0}},
			inward: [4]bool{true, false, false, false},
			err:    [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
		},
		testcase{ // 2
			line:   maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 100, Y: 0}},
			inward: [4]bool{true, false, false, false},
			err:    [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
		},
		testcase{ // 3
			line:   maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 150, Y: 0}},
			inward: [4]bool{true, false, false, false},
			err:    [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
		},

		// going the other way.
		testcase{ // 4
			line:    maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: -50, Y: 0}},
			inward:  [4]bool{false, false, true, false},
			err:     [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
			winding: maths.CounterClockwise,
		},
		testcase{ // 5
			line:    maths.Line{maths.Pt{X: 50, Y: 0}, maths.Pt{X: 0, Y: 0}},
			inward:  [4]bool{false, false, true, false},
			err:     [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
			winding: maths.CounterClockwise,
		},
		testcase{ // 6
			line:    maths.Line{maths.Pt{X: 100, Y: 0}, maths.Pt{X: 0, Y: 0}},
			inward:  [4]bool{false, false, true, false},
			err:     [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
			winding: maths.CounterClockwise,
		},
		testcase{ // 7
			line:    maths.Line{maths.Pt{X: 150, Y: 0}, maths.Pt{X: 0, Y: 0}},
			inward:  [4]bool{false, false, true, false},
			err:     [4]error{nil, ErrNoDirection, nil, ErrNoDirection},
			winding: maths.CounterClockwise,
		},

		// Vertical
		testcase{ // 8
			line:   maths.Line{maths.Pt{X: 0, Y: -50}, maths.Pt{X: 0, Y: 0}},
			inward: [4]bool{false, true, false, false},
			err:    [4]error{ErrNoDirection, nil, ErrNoDirection, nil},
		},
		testcase{ // 9
			line:   maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 50}},
			inward: [4]bool{false, true, false, false},
			err:    [4]error{ErrNoDirection, nil, ErrNoDirection, nil},
		},
		testcase{ // 10
			line:   maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 100}},
			inward: [4]bool{false, true, false, false},
			err:    [4]error{ErrNoDirection, nil, ErrNoDirection, nil},
		},
		testcase{ // 11
			line:   maths.Line{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 0, Y: 150}},
			inward: [4]bool{false, true, false, false},
			err:    [4]error{ErrNoDirection, nil, ErrNoDirection, nil},
		},

		// going the other way.
		testcase{ // 12
			line:    maths.Line{maths.Pt{X: 0, Y: 150}, maths.Pt{X: 0, Y: -50}},
			inward:  [4]bool{false, true, false, false},
			err:     [4]error{ErrNoDirection, nil, ErrNoDirection, nil},
			winding: maths.CounterClockwise,
		},
	)
	test.Run(func(idx int, tc testcase) {
		r := New(tc.winding, maths.Pt{X: 0, Y: 0}, maths.Pt{X: 100, Y: 100})
		for a, i := r.FirstAxis(), 0; a != nil; a, i = a.Next(), i+1 {
			inward, err := a.IsInward(tc.line)

			if inward != tc.inward[i] {
				t.Errorf("Test(%v) For Axis(%#v) IsInward[%v]: got %v want %v", idx, a, i, inward, tc.inward[i])
			}
			if tc.err[i] != err {
				t.Errorf("Test(%v) For Axis(%#v) Unexpected Err[%v]: Got %v want %v", idx, a, i, err, tc.err[i])
			}
		}
	})

}
