package makevalid

import (
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/maths"
	"github.com/gdey/tbltest"
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
			start:    []maths.Pt{{X: 25, Y: 19}, {X: 29, Y: 14}},
			pts:      []maths.Pt{{X: 25, Y: 19}, {X: 29, Y: 23}},
			expected: []maths.Pt{{X: 29, Y: 23}, {X: 25, Y: 19}, {X: 29, Y: 14}},
			added:    true,
		},
	)
	tests.Run(func(idx int, test testcase) {
		r := newRing(test.start)
		eadded := r.Add(test.pts)
		if eadded != test.added {
			t.Fatal("Added not equal")
		}
		if !reflect.DeepEqual(test.expected, r.r) {
			t.Fatal("Did not get expected.")
		}

	})

}
