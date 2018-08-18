package hitmap

import (
	"fmt"
	"log"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/maths"
)

func TestSegmentLinesContains(t *testing.T) {
	type tstPt struct {
		p         maths.Pt
		contained bool
	}
	type testcase struct {
		lines []maths.Line
		pts   []tstPt
		desc  string
	}

	cpt := func(x, y float64) tstPt { return tstPt{maths.Pt{x, y}, true} }
	ucpt := func(x, y float64) tstPt { return tstPt{maths.Pt{x, y}, false} }

	doesContain := func(pt tstPt) string {
		if pt.contained {
			return fmt.Sprintf("to contain Pt %v", pt.p)
		}
		return fmt.Sprintf("not to contain Pt %v", pt.p)
	}
	lines := func(xys ...float64) (lns []maths.Line) {
		lxi, lyi := len(xys)-2, len(xys)-1
		for xi, yi := 0, 1; yi < len(xys); lxi, lyi, xi, yi = xi, yi, xi+2, yi+2 {
			lns = append(lns, maths.Line{maths.Pt{xys[lxi], xys[lyi]}, maths.Pt{xys[xi], xys[yi]}})
		}
		return lns
	}

	tests := tbltest.Cases(
		testcase{
			desc:  "Simple square",
			lines: lines(7, 1, 7, 6, 3, 6, 3, 1),
			pts: []tstPt{
				cpt(3, 1), cpt(7, 1), cpt(7, 6), cpt(3, 6), cpt(4, 2), cpt(5, 3),
				ucpt(3, 0), ucpt(7, 0), ucpt(2, 6),
			},
		},
		// omb49G09l03lv$"ay'bo"0pVj@a
		// :s/1,/20,/g$o02j
		testcase{
			desc: "Complicated shape. (20x20)",
			lines: lines(
				2, 3, 4, 3, 4, 4, 6, 6, 9, 6, 8, 4, 6, 4,
				4, 2, 10, 2, 10, 4, 12, 6, 16, 3, 16, 4,
				18, 6, 18, 8, 16, 12, 14, 10, 16, 8, 16, 6,
				12, 11, 10, 8, 10, 7, 8, 7, 8, 10, 6, 10,
				6, 8, 4, 8, 4, 12, 18, 18, 8, 18, 2, 12,
				2, 8, 4, 6, 2, 4,
			),
			pts: []tstPt{
				ucpt(1, 1), ucpt(1, 2), ucpt(1, 3), ucpt(1, 4), ucpt(1, 5), ucpt(1, 6), ucpt(1, 7), ucpt(1, 8), ucpt(1, 9), ucpt(1, 10),
				ucpt(1, 11), ucpt(1, 12), ucpt(1, 13), ucpt(1, 14), ucpt(1, 15), ucpt(1, 16), ucpt(1, 17), ucpt(1, 18), ucpt(1, 19), ucpt(1, 20),

				ucpt(2, 1), ucpt(2, 2), cpt(2, 3), cpt(2, 4), ucpt(2, 5), ucpt(2, 6), ucpt(2, 7), cpt(2, 8), cpt(2, 9), cpt(2, 10),
				cpt(2, 11), cpt(2, 12), ucpt(2, 13), ucpt(2, 14), ucpt(2, 15), ucpt(2, 16), ucpt(2, 17), ucpt(2, 18), ucpt(2, 19), ucpt(2, 20),

				ucpt(3, 1), ucpt(3, 2), cpt(3, 3), cpt(3, 4), cpt(3, 5), ucpt(3, 6), cpt(3, 7), cpt(3, 8), cpt(3, 9), cpt(3, 10),
				cpt(3, 11), cpt(3, 12), cpt(3, 13), ucpt(3, 14), ucpt(3, 15), ucpt(3, 16), ucpt(3, 17), ucpt(3, 18), ucpt(3, 19), ucpt(3, 20),

				ucpt(4, 1), cpt(4, 2), cpt(4, 3), cpt(4, 4), cpt(4, 5), cpt(4, 6), cpt(4, 7), cpt(4, 8), cpt(4, 9), cpt(4, 10),
				cpt(4, 11), cpt(4, 12), cpt(4, 13), cpt(4, 14), ucpt(4, 15), ucpt(4, 16), ucpt(4, 17), ucpt(4, 18), ucpt(4, 19), ucpt(4, 20),

				ucpt(5, 1), cpt(5, 2), cpt(5, 3), ucpt(5, 4), cpt(5, 5), cpt(5, 6), cpt(5, 7), cpt(5, 8), ucpt(5, 9), ucpt(5, 10),
				ucpt(5, 11), ucpt(5, 12), cpt(5, 13), cpt(5, 14), cpt(5, 15), ucpt(5, 16), ucpt(5, 17), ucpt(5, 18), ucpt(5, 19), ucpt(5, 20),

				ucpt(6, 1), cpt(6, 2), cpt(6, 3), cpt(6, 4), ucpt(6, 5), cpt(6, 6), cpt(6, 7), cpt(6, 8), cpt(6, 9), cpt(6, 10),
				ucpt(6, 11), ucpt(6, 12), cpt(6, 13), cpt(6, 14), cpt(6, 15), cpt(6, 16), ucpt(6, 17), ucpt(6, 18), ucpt(6, 19), ucpt(6, 20),

				ucpt(7, 1), cpt(7, 2), cpt(7, 3), cpt(7, 4), ucpt(7, 5), cpt(7, 6), cpt(7, 7), cpt(7, 8), cpt(7, 9), cpt(7, 10),
				ucpt(7, 11), ucpt(7, 12), ucpt(7, 13), cpt(7, 14), cpt(7, 15), cpt(7, 16), cpt(7, 17), ucpt(7, 18), ucpt(7, 19), ucpt(7, 20),

				ucpt(8, 1), cpt(8, 2), cpt(8, 3), cpt(8, 4), ucpt(8, 5), cpt(8, 6), cpt(8, 7), cpt(8, 8), cpt(8, 9), cpt(8, 10),
				ucpt(8, 11), ucpt(8, 12), ucpt(8, 13), cpt(8, 14), cpt(8, 15), cpt(8, 16), cpt(8, 17), cpt(8, 18), ucpt(8, 19), ucpt(8, 20),

				ucpt(9, 1), cpt(9, 2), cpt(9, 3), cpt(9, 4), cpt(9, 5), cpt(9, 6), cpt(9, 7), ucpt(9, 8), ucpt(9, 9), ucpt(9, 10),
				ucpt(9, 11), ucpt(9, 12), ucpt(9, 13), ucpt(9, 14), cpt(9, 15), cpt(9, 16), cpt(9, 17), cpt(9, 18), ucpt(9, 19), ucpt(9, 20),

				ucpt(10, 1), cpt(10, 2), cpt(10, 3), cpt(10, 4), cpt(10, 5), cpt(10, 6), cpt(10, 7), cpt(10, 8), ucpt(10, 9), ucpt(10, 10),
				ucpt(10, 11), ucpt(10, 12), ucpt(10, 13), ucpt(10, 14), cpt(10, 15), cpt(10, 16), cpt(10, 17), cpt(10, 18), ucpt(10, 19), ucpt(10, 20),

				ucpt(11, 1), ucpt(11, 2), ucpt(11, 3), ucpt(11, 4), cpt(11, 5), cpt(11, 6), cpt(11, 7), cpt(11, 8), cpt(11, 9), ucpt(11, 10),
				ucpt(11, 11), ucpt(11, 12), ucpt(11, 13), ucpt(11, 14), cpt(11, 15), cpt(11, 16), cpt(11, 17), cpt(11, 18), ucpt(11, 19), ucpt(11, 20),

				ucpt(12, 1), ucpt(12, 2), ucpt(12, 3), ucpt(12, 4), ucpt(12, 5), cpt(12, 6), cpt(12, 7), cpt(12, 8), cpt(12, 9), cpt(12, 10),
				cpt(12, 11), ucpt(12, 12), ucpt(12, 13), ucpt(12, 14), ucpt(12, 15), cpt(12, 16), cpt(12, 17), cpt(12, 18), ucpt(12, 19), ucpt(12, 20),

				ucpt(13, 1), ucpt(13, 2), ucpt(13, 3), ucpt(13, 4), ucpt(13, 5), cpt(13, 6), cpt(13, 7), cpt(13, 8), cpt(13, 9), ucpt(13, 10),
				ucpt(13, 11), ucpt(13, 12), ucpt(13, 13), ucpt(13, 14), ucpt(13, 15), cpt(13, 16), cpt(13, 17), cpt(13, 18), ucpt(13, 19), ucpt(13, 20),

				ucpt(14, 1), ucpt(14, 2), ucpt(14, 3), ucpt(14, 4), cpt(14, 5), cpt(14, 6), cpt(14, 7), cpt(14, 8), ucpt(14, 9), cpt(14, 10),
				ucpt(14, 11), ucpt(14, 12), ucpt(14, 13), ucpt(14, 14), ucpt(14, 15), ucpt(14, 16), cpt(14, 17), cpt(14, 18), ucpt(14, 19), ucpt(14, 20),

				ucpt(15, 1), ucpt(15, 2), ucpt(15, 3), cpt(15, 4), cpt(15, 5), cpt(15, 6), cpt(15, 7), ucpt(15, 8), cpt(15, 9), cpt(15, 10),
				cpt(15, 11), ucpt(15, 12), ucpt(15, 13), ucpt(15, 14), ucpt(15, 15), ucpt(15, 16), cpt(15, 17), cpt(15, 18), ucpt(15, 19), ucpt(15, 20),

				ucpt(16, 1), ucpt(16, 2), cpt(16, 3), cpt(16, 4), cpt(16, 5), cpt(16, 6), cpt(16, 7), cpt(16, 8), cpt(16, 9), cpt(16, 10),
				cpt(16, 11), cpt(16, 12), ucpt(16, 13), ucpt(16, 14), ucpt(16, 15), ucpt(16, 16), ucpt(16, 17), cpt(16, 18), ucpt(16, 19), ucpt(16, 20),

				ucpt(17, 1), ucpt(17, 2), ucpt(17, 3), ucpt(17, 4), cpt(17, 5), cpt(17, 6), cpt(17, 7), cpt(17, 8), cpt(17, 9), cpt(17, 10),
				ucpt(17, 11), ucpt(17, 12), ucpt(17, 13), ucpt(17, 14), ucpt(17, 15), ucpt(17, 16), ucpt(17, 17), cpt(17, 18), ucpt(17, 19), ucpt(17, 20),

				ucpt(18, 1), ucpt(18, 2), ucpt(18, 3), ucpt(18, 4), ucpt(18, 5), cpt(18, 6), cpt(18, 7), cpt(18, 8), ucpt(18, 9), ucpt(18, 10),
				ucpt(18, 11), ucpt(18, 12), ucpt(18, 13), ucpt(18, 14), ucpt(18, 15), ucpt(18, 16), ucpt(18, 17), cpt(18, 18), ucpt(18, 19), ucpt(18, 20),

				ucpt(19, 1), ucpt(19, 2), ucpt(19, 3), ucpt(19, 4), ucpt(19, 5), ucpt(19, 6), ucpt(19, 7), ucpt(19, 8), ucpt(19, 9), ucpt(19, 10),
				ucpt(19, 11), ucpt(19, 12), ucpt(19, 13), ucpt(19, 14), ucpt(19, 15), ucpt(19, 16), ucpt(19, 17), ucpt(19, 18), ucpt(19, 19), ucpt(19, 20),

				ucpt(20, 1), ucpt(20, 2), ucpt(20, 3), ucpt(20, 4), ucpt(20, 5), ucpt(20, 6), ucpt(20, 7), ucpt(20, 8), ucpt(20, 9), ucpt(20, 10),
				ucpt(20, 11), ucpt(20, 12), ucpt(20, 13), ucpt(20, 14), ucpt(20, 15), ucpt(20, 16), ucpt(20, 17), ucpt(20, 18), ucpt(20, 19), ucpt(20, 20),
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		seg := NewSegmentFromLines(maths.Inside, test.lines)
		for _, pt := range test.pts {
			got := seg.Contains(pt.p)
			if got != pt.contained {
				log.Println("Testing: ", test.lines)
				t.Fatalf("For test( %v: %v ):Expected %v. got: %v ", idx, test.desc, doesContain(pt), got)
			}
		}
	})

}

func TestNewFromPolygon(t *testing.T) {
	// This just test to see if the new function panics for any reason.
	var tests map[string]basic.Polygon
	fn := func(t *testing.T, test basic.Polygon) {
		t.Parallel()
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic, expected nil got %v", r)
			}
		}()
		// Don't want it to optimized away.
		hm := NewFromPolygon(test)
		_ = hm
	}
	tests = map[string]basic.Polygon{
		"Nil Polygon":   nil,
		"Basic Polygon": {},
		"With One empty Line": {
			basic.Line{},
		},
		"With one non-empty line": {
			basic.Line{{10, 10}, {20, 10}, {20, 20}, {10, 20}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) { fn(t, test) })
	}
}

func TestNewFromMultiPolygon(t *testing.T) {
	// This just test to see if the new function panics for any reason.
	var tests map[string]basic.MultiPolygon
	fn := func(t *testing.T, test basic.MultiPolygon) {
		t.Parallel()
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic, expected nil got %v", r)
			}
		}()
		// Don't want it to optimized away.
		hm := NewFromMultiPolygon(test)
		_ = hm
	}
	tests = map[string]basic.MultiPolygon{
		"Nil Polygon":   nil,
		"Basic Polygon": {},
		"With One empty Line": {
			basic.Polygon{
				basic.Line{},
			},
		},
		"With one non-empty line": {
			basic.Polygon{
				basic.Line{{10, 10}, {20, 10}, {20, 20}, {10, 20}},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) { fn(t, test) })
	}
}
