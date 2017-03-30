package basic_test

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/basic"
)

func TestLineContainsPoint(t *testing.T) {
	type testcase struct {
		desc     string
		line     basic.Line
		point    basic.Point
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			desc:     "Simple circle with point(1,1)",
			line:     basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
			point:    basic.Point{1, 1},
			expected: true,
		},
		testcase{
			desc:     "Simple circle with point(2,2)",
			line:     basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
			point:    basic.Point{2, 2},
			expected: true,
		},
		testcase{
			desc:     "Simple circle with point(5,5) on border",
			line:     basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
			point:    basic.Point{5, 5},
			expected: false,
		},
		testcase{
			desc:     "Simple circle with point(6,6) outside.",
			line:     basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
			point:    basic.Point{6, 6},
			expected: false,
		},
	)
	tests.Run(func(idx int, test testcase) {
		got := test.line.Contains(test.point)
		if got != test.expected {
			t.Errorf("Tests %v (%v): Expected %v got %v", test.desc, idx, test.expected, got)
		}
	})
}

func TestLineContainsLine(t *testing.T) {
	type testcase struct {
		desc     string
		line     basic.Line
		cline    basic.Line
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			desc:     "small sqr(1 1, 3 3) fully contained in sqr(0 0, 5 8).",
			line:     basic.NewLine(0, 0, 5, 0, 5, 8, 0, 8),
			cline:    basic.NewLine(1, 1, 1, 3, 3, 3, 3, 1),
			expected: true,
		},
		testcase{
			desc:     "small sqr(0 0, 3 3) not fully contained in sqr(0 0, 5 8).",
			line:     basic.NewLine(0, 0, 5, 0, 5, 8, 0, 8),
			cline:    basic.NewLine(0, 0, 0, 3, 3, 3, 3, 0),
			expected: false,
		},
		testcase{
			desc:     "small sqr(-1 -1, 3 3) not fully contained in sqr(0 0, 5 8).",
			line:     basic.NewLine(0, 0, 5, 0, 5, 8, 0, 8),
			cline:    basic.NewLine(-1, -1, -1, 3, 3, 3, 3, -1),
			expected: false,
		},
		testcase{
			desc:     "small sqr(-3 -3, -1 -1) not fully contained in sqr(0 0, 5 8).",
			line:     basic.NewLine(0, 0, 5, 0, 5, 8, 0, 8),
			cline:    basic.NewLine(-3, -3, -3, -1, -1, -1, -1, -3),
			expected: false,
		},
		testcase{
			desc:     "small sqr(-3 -3, 0 0) not fully contained in sqr(0 0, 5 8).",
			line:     basic.NewLine(0, 0, 5, 0, 5, 8, 0, 8),
			cline:    basic.NewLine(-3, -3, -3, 0, 0, 0, 0, -3),
			expected: false,
		},
		testcase{
			desc:     "small sqr(-3 -3, 3 3) not fully contained in sqr(0 0, 5 8).",
			line:     basic.NewLine(0, 0, 5, 0, 5, 8, 0, 8),
			cline:    basic.NewLine(-3, -3, -3, 3, 3, 3, 3, -3),
			expected: false,
		},
	)
	tests.Run(func(idx int, test testcase) {
		got := test.line.ContainsLine(test.cline)
		if got != test.expected {
			t.Errorf("Tests %v (%v): expected: %v got %v", test.desc, idx, test.expected, got)
		}
	})
}
