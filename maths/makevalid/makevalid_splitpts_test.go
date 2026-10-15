package makevalid

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/maths"
	"github.com/alexeydott/tegola/maths/internal/assert"
	"github.com/gdey/tbltest"
)

func TestSplitPoints(t *testing.T) {
	type tcase struct {
		segs []maths.Line
		pts  [][]maths.Pt
		err  error
	}
	ctx := context.Background()
	fn := func(idx int, tc tcase) {
		pts, err := splitPoints(ctx, tc.segs)
		{
			e := assert.ErrorEquality(tc.err, err)
			// The errors are not equal for some reason.
			if e.Message != "" {
				t.Errorf("[%v] %v", idx, e)
			}
			if tc.err != nil {
				return
			}
		}
		if !reflect.DeepEqual(tc.pts, pts) {
			t.Errorf("[%v] %v", idx, assert.Equality{
				Message:  "split points",
				Expected: fmt.Sprint(tc.pts),
				Got:      fmt.Sprint(pts),
			})
		}
	}
	tbltest.Cases(
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 3, Y: 16}},
			},
			pts: [][]maths.Pt{
				{{X: 0, Y: 9}, {X: 2, Y: 13}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 2, Y: 13}, {X: 3, Y: 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 2, Y: 13}},
			},
			pts: [][]maths.Pt{
				{{X: 0, Y: 9}, {X: 2, Y: 13}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 2, Y: 13}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 2, Y: 13}},
				{{X: 0, Y: 7}, {X: 3, Y: 16}},
			},
			pts: [][]maths.Pt{
				{{X: 0, Y: 9}, {X: 2, Y: 13}},
				{{X: 0, Y: 7}, {X: 2, Y: 13}, {X: 3, Y: 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 3, Y: 16}},
				{{X: 0, Y: 5}, {X: 2, Y: 13}},
			},
			pts: [][]maths.Pt{
				{{X: 0, Y: 9}, {X: 2, Y: 13}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 2, Y: 13}, {X: 3, Y: 16}},
				{{X: 0, Y: 5}, {X: 2, Y: 13}},
			},
		},
	).Run(fn)
}
func TestSplitSegments(t *testing.T) {
	type tcase struct {
		segs    []maths.Line
		lns     [][2][2]float64
		clipbox *geom.Extent
		err     error
	}
	ctx := context.Background()
	fn := func(idx int, tc tcase) {
		lns, err := splitSegments(ctx, tc.segs, tc.clipbox)
		{
			e := assert.ErrorEquality(tc.err, err)
			// The errors are not equal for some reason.
			if e.Message != "" {
				t.Errorf("[%v] %v", idx, e)
			}
			if tc.err != nil {
				return
			}
		}
		if !reflect.DeepEqual(tc.lns, lns) {
			t.Errorf("[%v] %v", idx, assert.Equality{
				Message:  "split segments",
				Expected: fmt.Sprint(tc.lns),
				Got:      fmt.Sprint(lns),
			})
		}
	}
	tbltest.Cases(
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 3, Y: 16}},
			},
			lns: [][2][2]float64{
				{{0, 9}, {2, 13}},
				{{2, 13}, {4, 17}},
				{{0, 7}, {2, 13}},
				{{2, 13}, {3, 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 2, Y: 13}},
			},
			lns: [][2][2]float64{
				{{0, 9}, {2, 13}},
				{{2, 13}, {4, 17}},
				{{0, 7}, {2, 13}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 2, Y: 13}},
				{{X: 0, Y: 7}, {X: 3, Y: 16}},
			},
			lns: [][2][2]float64{
				{{0, 9}, {2, 13}},
				{{0, 7}, {2, 13}},
				{{2, 13}, {3, 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{X: 0, Y: 9}, {X: 4, Y: 17}},
				{{X: 0, Y: 7}, {X: 3, Y: 16}},
				{{X: 0, Y: 5}, {X: 2, Y: 13}},
			},
			lns: [][2][2]float64{
				{{0, 9}, {2, 13}},
				{{2, 13}, {4, 17}},
				{{0, 7}, {2, 13}},
				{{2, 13}, {3, 16}},
				{{0, 5}, {2, 13}},
			},
		},
	).Run(fn)
}
