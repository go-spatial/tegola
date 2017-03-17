package maths_test

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
)

func invertPoints(pts []float64) (rpts []float64) {
	rpts = append(rpts, pts[0], pts[1])
	for x, y := len(pts)-2, len(pts)-1; x > 0; x, y = x-2, y-2 {
		rpts = append(rpts, pts[x], pts[y])
	}
	return rpts
}

func TestWindingOrderOf(t *testing.T) {

	type testcase struct {
		points   []float64
		expected maths.WindingOrder
	}
	tests := tbltest.Cases(
		testcase{
			points:   []float64{4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2},
			expected: maths.CounterClockwise,
		},
	)

	tests.Run(func(idx int, test testcase) {
		got := maths.WindingOrderOf(test.points)
		if got != test.expected {
			t.Errorf("Test %v: Failed expected %v got %v", idx, test.expected, got)
		}
		got = maths.WindingOrderOfLine(basic.NewLine(test.points...))
		if got != test.expected {
			t.Errorf("Test %v: Failed for Line expected %v got %v", idx, test.expected, got)
		}
		ipts := invertPoints(test.points)
		got = maths.WindingOrderOf(ipts)
		if got != test.expected.Not() {
			t.Errorf("Test %v Inverted: Failed expected %v got %v", idx, test.expected.Not(), got)
		}
		got = maths.WindingOrderOfLine(basic.NewLine(ipts...))
		if got != test.expected.Not() {
			t.Errorf("Test %v Inverted: Failed for Line expected %v got %v", idx, test.expected.Not(), got)
		}
	})
}
