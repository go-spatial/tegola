package makevalid

import (
	"log"
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
)

func TestConstuctRing(t *testing.T) {
	type testcase struct {
		start    []maths.Pt
		pts      []maths.Pt
		expected []maths.Pt
		added    bool
	}

	tests := tbltest.Cases(
		testcase{
			start:    []maths.Pt{{25, 19}, {29, 14}},
			pts:      []maths.Pt{{25, 19}, {29, 23}},
			expected: []maths.Pt{{29, 23}, {25, 19}, {29, 14}},
			added:    true,
		},
	)
	tests.Run(func(idx int, test testcase) {
		r := newRing(test.start)
		eadded := r.Add(test.pts)
		log.Println(r, eadded)
		if eadded != test.added {
			t.Fatal("Added not equal")
		}
		if !reflect.DeepEqual(test.expected, r.r) {
			t.Fatal("Did not get expected.")
		}

	})

}
