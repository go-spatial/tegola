package makevalid

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/internal/assert"
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
				{{0, 9}, {4, 17}},
				{{0, 7}, {3, 16}},
			},
			pts: [][]maths.Pt{
				[]maths.Pt{{0, 9}, {2, 13}, {4, 17}},
				[]maths.Pt{{0, 7}, {2, 13}, {3, 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{0, 9}, {4, 17}},
				{{0, 7}, {2, 13}},
			},
			pts: [][]maths.Pt{
				[]maths.Pt{{0, 9}, {2, 13}, {4, 17}},
				[]maths.Pt{{0, 7}, {2, 13}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{0, 9}, {2, 13}},
				{{0, 7}, {3, 16}},
			},
			pts: [][]maths.Pt{
				[]maths.Pt{{0, 9}, {2, 13}},
				[]maths.Pt{{0, 7}, {2, 13}, {3, 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{0, 9}, {4, 17}},
				{{0, 7}, {3, 16}},
				{{0, 5}, {2, 13}},
			},
			pts: [][]maths.Pt{
				[]maths.Pt{{0, 9}, {2, 13}, {4, 17}},
				[]maths.Pt{{0, 7}, {2, 13}, {3, 16}},
				[]maths.Pt{{0, 5}, {2, 13}},
			},
		},
	).Run(fn)
}
func TestSplitSegments(t *testing.T) {
	type tcase struct {
		segs    []maths.Line
		lns     [][2][2]float64
		clipbox *geom.BoundingBox
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
				{{0, 9}, {4, 17}},
				{{0, 7}, {3, 16}},
			},
			lns: [][2][2]float64{
				[2][2]float64{{0, 9}, {2, 13}},
				[2][2]float64{{2, 13}, {4, 17}},
				[2][2]float64{{0, 7}, {2, 13}},
				[2][2]float64{{2, 13}, {3, 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{0, 9}, {4, 17}},
				{{0, 7}, {2, 13}},
			},
			lns: [][2][2]float64{
				[2][2]float64{{0, 9}, {2, 13}},
				[2][2]float64{{2, 13}, {4, 17}},
				[2][2]float64{{0, 7}, {2, 13}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{0, 9}, {2, 13}},
				{{0, 7}, {3, 16}},
			},
			lns: [][2][2]float64{
				[2][2]float64{{0, 9}, {2, 13}},
				[2][2]float64{{0, 7}, {2, 13}},
				[2][2]float64{{2, 13}, {3, 16}},
			},
		},
		tcase{
			segs: []maths.Line{
				{{0, 9}, {4, 17}},
				{{0, 7}, {3, 16}},
				{{0, 5}, {2, 13}},
			},
			lns: [][2][2]float64{
				[2][2]float64{{0, 9}, {2, 13}},
				[2][2]float64{{2, 13}, {4, 17}},
				[2][2]float64{{0, 7}, {2, 13}},
				[2][2]float64{{2, 13}, {3, 16}},
				[2][2]float64{{0, 5}, {2, 13}},
			},
		},
	).Run(fn)
}
