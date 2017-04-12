package intersect

import (
	"log"
	"testing"

	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/clip/region"
	"github.com/terranodo/tegola/maths/clip/subject"
)

func TestNewIntersect(t *testing.T) {
	pt := NewPt(maths.Pt{0, 5}, true)
	l := New()
	l.PushBack(pt)
	rl := region.New(maths.CounterClockwise, maths.Pt{0, 0}, maths.Pt{10, 10})
	rl.Axis(0).PushInBetween(pt.AsRegionPoint())
	sl, err := subject.New([]float64{0, 0, 0, 10, 10, 10})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
		return
	}
	sl.PushInBetween(sl.Front().(list.ElementerPointer), sl.Front().Next().(list.ElementerPointer), pt.AsSubjectPoint())
	log.Printf("intersect: %#v\n", l)
	log.Printf("region: %#v\n", rl)
	log.Printf("subject: %#v\n", sl)
	for ib := l.FirstInboundPtWalker(); ib != nil; ib = ib.Next() {
		log.Printf("InBound %#v\n", ib)
		ib.Walk(func(idx int, pt maths.Pt) bool {
			log.Printf("Pt %v: %v\n", idx, pt)
			if idx == 10 {
				return false
			}
			return true
		})
	}

}
