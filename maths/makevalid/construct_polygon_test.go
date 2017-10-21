package makevalid

import (
	"log"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
)

func TestConstuctPolygon(t *testing.T) {
	type testcase struct {
		lines []maths.Line
	}
	tests := tbltest.Cases(
		testcase{
			lines: []maths.Line{
				maths.Line{maths.Pt{X: 25, Y: 19}, maths.Pt{X: 29, Y: 14}},
				maths.Line{maths.Pt{X: 25, Y: 19}, maths.Pt{X: 29, Y: 23}},
				maths.Line{maths.Pt{X: 29, Y: 14}, maths.Pt{X: 32, Y: 14}},
				maths.Line{maths.Pt{X: 29, Y: 23}, maths.Pt{X: 32, Y: 25}},
				maths.Line{maths.Pt{X: 32, Y: 14}, maths.Pt{X: 36, Y: 17}},
				maths.Line{maths.Pt{X: 36, Y: 17}, maths.Pt{X: 36, Y: 20}},
				maths.Line{maths.Pt{X: 32, Y: 25}, maths.Pt{X: 36, Y: 29}},
				maths.Line{maths.Pt{X: 36, Y: 20}, maths.Pt{X: 44, Y: 30}},
				maths.Line{maths.Pt{X: 36, Y: 29}, maths.Pt{X: 44, Y: 37}},
				maths.Line{maths.Pt{X: 44, Y: 30}, maths.Pt{X: 47, Y: 32}},
				maths.Line{maths.Pt{X: 44, Y: 37}, maths.Pt{X: 47, Y: 39}},
				maths.Line{maths.Pt{X: 47, Y: 32}, maths.Pt{X: 48, Y: 33}},
				maths.Line{maths.Pt{X: 47, Y: 39}, maths.Pt{X: 48, Y: 40}},
				maths.Line{maths.Pt{X: 48, Y: 33}, maths.Pt{X: 50, Y: 34}},
				maths.Line{maths.Pt{X: 48, Y: 40}, maths.Pt{X: 50, Y: 42}},
				maths.Line{maths.Pt{X: 50, Y: 34}, maths.Pt{X: 51, Y: 34}},
				maths.Line{maths.Pt{X: 50, Y: 42}, maths.Pt{X: 51, Y: 43}},
				maths.Line{maths.Pt{X: 51, Y: 34}, maths.Pt{X: 52, Y: 35}},
				maths.Line{maths.Pt{X: 51, Y: 43}, maths.Pt{X: 52, Y: 44}},
				maths.Line{maths.Pt{X: 52, Y: 35}, maths.Pt{X: 53, Y: 36}},
				maths.Line{maths.Pt{X: 51, Y: 60}, maths.Pt{X: 52, Y: 60}},
				maths.Line{maths.Pt{X: 52, Y: 60}, maths.Pt{X: 53, Y: 61}},
				maths.Line{maths.Pt{X: 53, Y: 36}, maths.Pt{X: 58, Y: 39}},
				maths.Line{maths.Pt{X: 50, Y: 60}, maths.Pt{X: 51, Y: 59}},
				maths.Line{maths.Pt{X: 50, Y: 60}, maths.Pt{X: 51, Y: 60}},
				maths.Line{maths.Pt{X: 51, Y: 46}, maths.Pt{X: 52, Y: 44}},
				maths.Line{maths.Pt{X: 53, Y: 61}, maths.Pt{X: 58, Y: 54}},
				maths.Line{maths.Pt{X: 50, Y: 58}, maths.Pt{X: 51, Y: 59}},
				maths.Line{maths.Pt{X: 58, Y: 39}, maths.Pt{X: 66, Y: 25}},
				maths.Line{maths.Pt{X: 50, Y: 48}, maths.Pt{X: 51, Y: 46}},
				maths.Line{maths.Pt{X: 58, Y: 54}, maths.Pt{X: 66, Y: 42}},
				maths.Line{maths.Pt{X: 48, Y: 56}, maths.Pt{X: 50, Y: 58}},
				maths.Line{maths.Pt{X: 66, Y: 25}, maths.Pt{X: 71, Y: 18}},
				maths.Line{maths.Pt{X: 48, Y: 53}, maths.Pt{X: 50, Y: 48}},
				maths.Line{maths.Pt{X: 66, Y: 42}, maths.Pt{X: 71, Y: 35}},
				maths.Line{maths.Pt{X: 47, Y: 55}, maths.Pt{X: 48, Y: 53}},
				maths.Line{maths.Pt{X: 47, Y: 55}, maths.Pt{X: 48, Y: 56}},
				maths.Line{maths.Pt{X: 71, Y: 18}, maths.Pt{X: 74, Y: 14}},
				maths.Line{maths.Pt{X: 71, Y: 35}, maths.Pt{X: 74, Y: 31}},
				maths.Line{maths.Pt{X: 74, Y: 14}, maths.Pt{X: 75, Y: 13}},
				maths.Line{maths.Pt{X: 74, Y: 31}, maths.Pt{X: 75, Y: 29}},
				maths.Line{maths.Pt{X: 75, Y: 29}, maths.Pt{X: 77, Y: 26}},
				maths.Line{maths.Pt{X: 75, Y: 1}, maths.Pt{X: 77, Y: 0}},
				maths.Line{maths.Pt{X: 77, Y: 26}, maths.Pt{X: 84, Y: 16}},
				maths.Line{maths.Pt{X: 77, Y: 0}, maths.Pt{X: 84, Y: 16}},
				maths.Line{maths.Pt{X: 74, Y: 10}, maths.Pt{X: 75, Y: 13}},
				maths.Line{maths.Pt{X: 74, Y: 1}, maths.Pt{X: 75, Y: 1}},
				maths.Line{maths.Pt{X: 71, Y: 8}, maths.Pt{X: 74, Y: 1}},
				maths.Line{maths.Pt{X: 71, Y: 8}, maths.Pt{X: 74, Y: 10}},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		got := constructPolygon(test.lines)
		for i := range got {
			log.Printf("Ring(%v):%v", i, got[i])
		}
	})
}
