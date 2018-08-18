package plyg

import (
	"log"
	"reflect"
	"sort"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/maths"
)

func TestBuilder(t *testing.T) {
	type testcase struct {
		desc    string
		ipoints [][2][]float64
		ring    Ring
	}

	tests := tbltest.Cases(
		testcase{
			desc: "Simple Triangle.",
			ipoints: [][2][]float64{
				{{0, 1}, {0}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 0}, {1, 0}, {0, 1}},
			},
		},
		testcase{
			desc: "Simple Triangle two.",
			ipoints: [][2][]float64{
				{{0}, {0, 1}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 0}, {1, 0}, {1, 1}},
			},
		},
		testcase{
			desc: "Simple Square.",
			ipoints: [][2][]float64{
				{{0}, {0, 1}},
				{{0, 1}, {1}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			},
		},
		testcase{
			desc: "Diag Rect.",
			ipoints: [][2][]float64{
				{{0, 1}, {1}},
				{{1}, {1, 2}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 0}, {1, 1}, {1, 2}, {0, 1}},
			},
		},
		testcase{
			desc: "Diag Rect. 1",
			ipoints: [][2][]float64{
				{{1}, {0, 1}},
				{{1, 2}, {1}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 1}, {1, 0}, {1, 1}, {0, 2}},
			},
		},
		testcase{
			desc: "Large Triangle.",
			ipoints: [][2][]float64{
				{{0, 1}, {1}},
				{{1, 2}, {1}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 0}, {1, 1}, {0, 2}, {0, 1}},
			},
		},
		testcase{
			desc: "Large Triangle 1.",
			ipoints: [][2][]float64{
				{{1}, {0, 1}},
				{{1}, {1, 2}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 1}, {1, 0}, {1, 1}, {1, 2}},
			},
		},
		testcase{
			desc: "Left Triangle and Rectangle",
			ipoints: [][2][]float64{
				{{0, 1}, {1}},
				{{1}, {1, 2}},
				{{1, 2}, {2}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 0}, {1, 1}, {1, 2}, {0, 2}, {0, 1}},
			},
		},
		testcase{
			desc: "Right Triangle and Rectangle",
			ipoints: [][2][]float64{
				{{1}, {0, 1}},
				{{1}, {1, 2}},
				{{1, 2}, {2}},
			},
			ring: Ring{
				Label:  maths.Inside,
				Points: []maths.Pt{{0, 1}, {1, 0}, {1, 1}, {1, 2}, {0, 2}},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		var b Builder
		var ys [2][]YPart
		var xs [2]float64
		log.Printf("Running %v (%v)", idx, test.desc)
		xs[0] = test.ring.Points[0].X
		xs[1] = test.ring.Points[1].X
		for i := range test.ipoints {
			var pts1, pts2 []maths.Pt
			// We are going to ignore the new bool for now, as we are only
			// testing if it can produce one ring.
			for _, y := range test.ipoints[i][0] {
				pts1 = append(pts1, maths.Pt{xs[0], y})
			}
			for _, y := range test.ipoints[i][1] {
				pts2 = append(pts2, maths.Pt{xs[1], y})
			}
			b.AddPts(test.ring.Label, pts1, pts2)
		}
		ring, x1, y1s, x2, y2s := b.CurrentRing()
		for i, pt := range test.ring.Points {
			if pt.X == xs[0] {
				ys[0] = append(ys[0], YPart{Y: pt.Y, Idx: i})
			} else {
				ys[1] = append(ys[1], YPart{Y: pt.Y, Idx: i})
			}
		}
		sort.Sort(YPartByY(ys[0]))
		sort.Sort(YPartByY(ys[1]))

		if !reflect.DeepEqual(ring, test.ring) {
			t.Errorf("For %v (%v) Ring:\nGot:\n\t%v\nExpected:\n\t%v", idx, test.desc, ring, test.ring)
		}
		if x1 != xs[0] {
			t.Errorf("For %v (%v) x1:\nGot:\n\t%v\nExpected:\n\t%v", idx, test.desc, x1, xs[0])
		}
		if x2 != xs[1] {
			t.Errorf("For %v (%v) x2:\nGot:\n\t%v\nExpected:\n\t%v", idx, test.desc, x2, xs[1])
		}
		if !reflect.DeepEqual(y1s, ys[0]) {
			t.Errorf("For %v (%v) y1s:\nGot:\n\t%v\nExpected:\n\t%v", idx, test.desc, y1s, ys[0])
		}
		if !reflect.DeepEqual(y2s, ys[1]) {
			t.Errorf("For %v (%v) y2s:\nGot:\n\t%v\nExpected:\n\t%v", idx, test.desc, y2s, ys[1])
		}
	})
}
