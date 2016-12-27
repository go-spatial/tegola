package subject

import (
	"testing"

	"github.com/terranodo/tegola/maths"
)

func TestNewSubject(t *testing.T) {
	sub, err := New(maths.CounterClockwise, []float64{0, 0, 1, 1, 2, 10, 10, 10})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
		return
	}
	exp := [][2]maths.Pt{
		[2]maths.Pt{maths.Pt{X: 10, Y: 10}, maths.Pt{X: 0, Y: 0}}, [2]maths.Pt{maths.Pt{X: 0, Y: 0}, maths.Pt{X: 1, Y: 1}},
		[2]maths.Pt{maths.Pt{X: 1, Y: 1}, maths.Pt{X: 2, Y: 10}},
		[2]maths.Pt{maths.Pt{X: 2, Y: 10}, maths.Pt{X: 10, Y: 10}},
	}
	for p, i := sub.FirstPair(), 0; p != nil; p, i = p.Next(), i+1 {
		pt1 := p.Pt1().Point()
		pt2 := p.Pt2().Point()
		if !exp[i][0].IsEqual(pt1) || !exp[i][1].IsEqual(pt2) {
			t.Errorf("NewSubject: For subject point(%v) Got %v,%v want %v", i, pt1, pt2, exp[0])
		}
	}
}
