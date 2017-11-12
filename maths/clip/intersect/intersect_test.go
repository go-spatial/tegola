package intersect

import (
	"testing"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/clip/region"
	"github.com/terranodo/tegola/maths/clip/subject"
)

func TestNewIntersect(t *testing.T) {

	// Counter Clockwise subject.
	sl, err := subject.New([]float64{-5, -5, -5, 5, 5, 5, 5, -5})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
		return
	}
	rl := region.New(sl.Winding(), maths.Pt{0, 0}, maths.Pt{10, 10})
	/*
		log.Printf("region: %#v\n", rl)
		log.Printf("subject: %#v\n", sl)
	*/
	l := New()
	inwardPt := NewPt(maths.Pt{0, 5}, true)
	l.PushBack(inwardPt)
	rl.Axis(0).PushInBetween(inwardPt.AsRegionPoint())
	if slp := sl.GetPair(2); slp != nil {
		slp.PushInBetween(inwardPt.AsSubjectPoint())
	}

	outwardPt := NewPt(maths.Pt{5, 0}, false)
	l.PushBack(outwardPt)
	rl.Axis(3).PushInBetween(outwardPt.AsRegionPoint())
	if slp := sl.GetPair(3); slp != nil {
		slp.PushInBetween(outwardPt.AsSubjectPoint())
	}
	/*
		log.Printf("intersect: %#v\n", l)
		log.Printf("region: %#v\n", rl)
		log.Printf("subject: %#v\n", sl)
	*/
	expectedWalk := [][]maths.Pt{
		[]maths.Pt{
			maths.Pt{0, 5}, maths.Pt{5, 5}, maths.Pt{5, 0}, maths.Pt{0, 0},
		},
	}
	current := 0
	for ib := l.FirstInboundPtWalker(); ib != nil; ib = ib.Next() {
		if len(expectedWalk) <= current {
			t.Fatalf("Too many paths: expected: %v got: %v", len(expectedWalk), current)
		}
		//log.Printf("InBound %#v\n", ib)
		ib.Walk(func(idx int, pt maths.Pt) bool {
			//log.Printf("Pt %v: %v\n", idx, pt)
			if len(expectedWalk[current]) <= idx {
				t.Fatalf("Too many points for (%v): expected: %v got: %v", current, len(expectedWalk[current]), idx)
			}
			if !expectedWalk[current][idx].IsEqual(pt) {
				t.Errorf("Point(%v) not correct of line %v: Expected: %v got %v", idx, current, expectedWalk[current][idx], pt)

			}
			if idx == 10 {
				t.Error("More then 10 paths returned!!")
				return false
			}
			return true
		})
		current++
	}

}
