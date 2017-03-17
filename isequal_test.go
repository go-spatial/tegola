package tegola_test

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

func TestIsPointEqual(t *testing.T) {
	type testcase struct {
		desc     string
		pt1      tegola.Point
		pt2      tegola.Point
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			desc: "Simple Points not equal",
			pt1:  basic.Point{1, 1},
			pt2:  basic.Point{2, 2},
		},
		testcase{
			desc: "Simple Points not equal",
			pt1:  basic.Point{1, 1},
			pt2:  basic.Point{1, 2},
		},
		testcase{
			desc: "Simple Points not equal",
			pt1:  basic.Point{1, 1},
			pt2:  basic.Point{2, 1},
		},
		testcase{
			desc: "One nil and one nonnil Points not equal",
			pt1:  basic.Point{1, 1},
		},
		testcase{
			desc: "One nil and one nonnil Points not equal",
			pt2:  basic.Point{1, 1},
		},
		testcase{
			desc:     "Simple Points are equal",
			pt1:      basic.Point{1, 1},
			pt2:      basic.Point{1, 1},
			expected: true,
		},
		testcase{
			desc:     "nil Points are equal",
			expected: true,
		},
	)
	tests.Run(func(test testcase) {
		if test.expected != tegola.IsPointEqual(test.pt1, test.pt2) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Points( %v, %v ) to %v", test.pt1, test.pt2, match)
		}
		if test.expected != tegola.IsPointEqual(test.pt2, test.pt1) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Points( %v, %v ) to %v", test.pt2, test.pt1, match)
		}
	})
}

func TestIsMultiPointEqual(t *testing.T) {
	type testcase struct {
		desc     string
		mpt1     basic.MultiPoint
		mpt2     basic.MultiPoint
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			desc:     "Simple single point, the same.",
			expected: true,
			mpt1:     basic.MultiPoint{basic.Point{1, 1}},
			mpt2:     basic.MultiPoint{basic.Point{1, 1}},
		},
		testcase{
			desc:     "Simple single point, the same.",
			expected: true,
			mpt1:     basic.MultiPoint{basic.Point{1, 1}, basic.Point{2, 2}},
			mpt2:     basic.MultiPoint{basic.Point{1, 1}, basic.Point{2, 2}},
		},
		testcase{
			desc:     "Simple nil points are the same.",
			expected: true,
		},
		testcase{
			desc: "multipoint and nil is not the same.",
			mpt1: basic.MultiPoint{basic.Point{1, 1}, basic.Point{2, 2}},
		},
		testcase{
			desc: "Different points are not the same.",
			mpt1: basic.MultiPoint{basic.Point{1, 1}, basic.Point{2, 2}},
			mpt2: basic.MultiPoint{basic.Point{-1, 1}, basic.Point{2, 2}},
		},
		testcase{
			desc: "Different points are not the same.",
			mpt1: basic.MultiPoint{basic.Point{1, 1}, basic.Point{2, 2}},
			mpt2: basic.MultiPoint{basic.Point{1, 1}, basic.Point{-2, 2}},
		},
		testcase{
			desc: "Different points are not the same.",
			mpt1: basic.MultiPoint{basic.Point{1, 1}},
			mpt2: basic.MultiPoint{basic.Point{1, 1}, basic.Point{2, 2}},
		},
	)
	tests.Run(func(test testcase) {
		if test.expected != tegola.IsMultiPointEqual(test.mpt1, test.mpt2) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected MultiPoints( %v, %v ) to %v", test.mpt1, test.mpt2, match)
		}
		if test.expected != tegola.IsMultiPointEqual(test.mpt2, test.mpt1) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected MultiPoints( %v, %v ) to %v", test.mpt2, test.mpt1, match)
		}
	})
}

func TestIsLineEqual(t *testing.T) {
	type testcase struct {
		Desc     string
		ln1      basic.Line
		ln2      basic.Line
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			Desc:     "Simple lines",
			ln1:      basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:      basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			expected: true,
		},
		testcase{
			Desc: "Simple lines that don't match wrong length.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3),
		},
		testcase{
			Desc: "Simple lines that don't match wrong length.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(1, 1, 2, 2, 3, 3, 4, 4),
		},
		testcase{
			Desc: "Simple lines that don't match wrong length.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(0, 0, 1, 1, 3, 3, 4, 4),
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(-1, -1, 1, 1, 2, 2, 3, 3, 4, 4),
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, -4, -4),
		},
		testcase{
			Desc: "Simple lines that don't match; numbers are off.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(1, 0, 1, 1, 2, 2, 3, 3, 4, 4),
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, -4),
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			ln1:  basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			ln2:  basic.NewLine(0, 0, 1, 1, 2, -2, 3, 3, 4, 4),
		},
	)
	tests.Run(func(test testcase) {
		if test.expected != tegola.IsLineStringEqual(test.ln1, test.ln2) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Lines( %v, %v ) to %v", test.ln1, test.ln2, match)
		}
		if test.expected != tegola.IsLineStringEqual(test.ln2, test.ln1) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Lines( %v, %v ) to %v", test.ln2, test.ln1, match)
		}
	})
}

func TestIsMultiLineEqual(t *testing.T) {
	type testcase struct {
		Desc     string
		lns1     basic.MultiLine
		lns2     basic.MultiLine
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			Desc:     "Simple lines",
			lns1:     basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2:     basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			expected: true,
		},
		testcase{
			Desc: "Simple lines",
			lns1: basic.MultiLine{
				basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
				basic.NewLine(5, 0, 5, 1, 5, 2, 5, 3, 5, 4),
			},
			lns2: basic.MultiLine{
				basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
				basic.NewLine(5, 0, 5, 1, 5, 2, 5, 3, 5, 4),
			},
			expected: true,
		},
		testcase{
			Desc: "Simple lines",
			lns1: basic.MultiLine{
				basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
				basic.NewLine(5, 0, 5, 1, 5, 2, 5, 3, 5, 4),
			},
			lns2: basic.MultiLine{
				basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
				basic.NewLine(5, 0, 5, 1, 5, 2, 5, 3),
			},
		},
		testcase{
			Desc: "Simple lines",
			lns1: basic.MultiLine{
				basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
			},
			lns2: basic.MultiLine{
				basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4),
				basic.NewLine(5, 0, 5, 1, 5, 2, 5, 3, 5, 4),
			},
		},
		testcase{
			Desc: "Simple lines that don't match wrong length.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3)},
		},
		testcase{
			Desc: "Simple lines that don't match wrong length.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(1, 1, 2, 2, 3, 3, 4, 4)},
		},
		testcase{
			Desc: "Simple lines that don't match wrong length.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 3, 3, 4, 4)},
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(-1, -1, 1, 1, 2, 2, 3, 3, 4, 4)},
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, -4, -4)},
		},
		testcase{
			Desc: "Simple lines that don't match; numbers are off.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(1, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, -4)},
		},
		testcase{
			Desc: "Simple lines that don't matc; numbers are off.",
			lns1: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, 2, 3, 3, 4, 4)},
			lns2: basic.MultiLine{basic.NewLine(0, 0, 1, 1, 2, -2, 3, 3, 4, 4)},
		},
	)
	tests.Run(func(test testcase) {
		if test.expected != tegola.IsMultiLineEqual(test.lns1, test.lns2) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Lines( %v, %v ) to %v", test.lns1, test.lns2, match)
		}
		if test.expected != tegola.IsMultiLineEqual(test.lns2, test.lns1) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Lines( %v, %v ) to %v", test.lns2, test.lns1, match)
		}
	})
}

func TestIsPolygonEqual(t *testing.T) {
	type testcase struct {
		Desc     string
		poly1    basic.Polygon
		poly2    basic.Polygon
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			Desc:     "Simple lines",
			poly1:    basic.Polygon{basic.NewLine(0, 0, 1, 0, 0, 1)},
			poly2:    basic.Polygon{basic.NewLine(0, 0, 1, 0, 0, 1)},
			expected: true,
		},
		testcase{
			Desc: "Simple lines",
			poly1: basic.Polygon{
				basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
				basic.NewLine(0, 0, 0, 3, 3, 3, 3, 0),
			},
			poly2: basic.Polygon{
				basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
				basic.NewLine(0, 0, 0, 3, 3, 3, 3, 0),
			},
			expected: true,
		},
		testcase{
			Desc: "Simple lines",
			poly1: basic.Polygon{
				basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
				basic.NewLine(0, 0, 0, 3, 3, 3, 3, 0),
			},
			poly2: basic.Polygon{
				basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
				basic.NewLine(0, 0, 0, 3, 3, 3),
			},
		},
		testcase{
			Desc: "Simple lines",
			poly1: basic.Polygon{
				basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
				basic.NewLine(0, 0, 0, 3, 3, 3, 3, 0),
			},
			poly2: basic.Polygon{
				basic.NewLine(0, 0, 5, 0, 5, 5, 0, 5),
			},
		},
	)
	tests.Run(func(test testcase) {
		if test.expected != tegola.IsPolygonEqual(test.poly1, test.poly2) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Lines( %v, %v ) to %v", test.poly1, test.poly2, match)
		}
		if test.expected != tegola.IsPolygonEqual(test.poly2, test.poly1) {
			match := "match."
			if !test.expected {
				match = "not " + match
			}
			t.Errorf("Expected Lines( %v, %v ) to %v", test.poly2, test.poly1, match)
		}
	})
}
