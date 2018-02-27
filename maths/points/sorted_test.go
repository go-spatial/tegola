package points

import (
	"log"
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func TestSortAndUnique(t *testing.T) {
	type tcase struct {
		uspts []maths.Pt
		spts  []maths.Pt
	}
	fn := func(idx int, tc tcase) {
		gspts := SortAndUnique(tc.uspts)
		if !reflect.DeepEqual(tc.spts, gspts) {
			t.Errorf("[%v] did not sort and unique, Expected %v Got %v", idx, tc.spts, gspts)
		}
	}
	tbltest.Cases(
		tcase{},
		tcase{
			uspts: []maths.Pt{{1, 2}},
			spts:  []maths.Pt{{1, 2}},
		},
		tcase{
			uspts: []maths.Pt{{1, 2}, {1, 2}},
			spts:  []maths.Pt{{1, 2}},
		},
		tcase{
			uspts: []maths.Pt{{1, 2}, {1, 2}, {3, 4}, {5, 6}, {5, 6}},
			spts:  []maths.Pt{{1, 2}, {3, 4}, {5, 6}},
		},
		tcase{
			uspts: []maths.Pt{{7, 8}, {1, 2}, {3, 4}, {5, 6}, {3, 4}, {1, 2}, {7, 8}, {2, 3}, {1, 2}},
			spts:  []maths.Pt{{1, 2}, {2, 3}, {3, 4}, {5, 6}, {7, 8}},
		},
	).Run(fn)
}
