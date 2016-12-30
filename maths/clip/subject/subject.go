package subject

import (
	"fmt"

	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

type Subject struct {
	winding maths.WindingOrder
	list.List
}

func New(winding maths.WindingOrder, coords []float64) (*Subject, error) {
	return new(Subject).Init(winding, coords)
}

func (s *Subject) Init(winding maths.WindingOrder, coords []float64) (*Subject, error) {
	s.winding = winding
	s.List.Init()
	if len(coords)%2 != 0 {
		return nil, fmt.Errorf("Even number of coords expected.")
	}
	for x, y := 0, 1; x < len(coords); x, y = x+2, y+2 {
		s.PushBack(list.NewPoint(coords[x], coords[y]))
	}
	return s, nil
}

func (s *Subject) FirstPair() *pair {
	if s == nil {
		return nil
	}
	var first, last *list.Pt
	var ok bool
	l, f := s.Back(), s.Front()
	if last, ok = l.(*list.Pt); !ok {
		return nil
	}
	if first, ok = f.(*list.Pt); !ok {
		return nil
	}
	return &pair{
		l:   &(s.List),
		pts: [2]*list.Pt{last, first},
	}

}

// Contains will test to see if the point if fully contained by the subject. If the point is on the broader it is not considered as contained.
func (s *Subject) Contains(pt maths.Pt) bool {
	line := maths.Line{pt, maths.Pt{pt.X - 1, pt.Y}}
	count := 0
	for p := s.FirstPair(); p != nil; p = p.Next() {
		pline := p.AsLine()
		if ipt, ok := maths.Intersect(line, pline); ok {
			// We only care about intersect points that are left of the point being tested.
			if pline.InBetween(ipt) && ipt.X < pt.X {
				count++
			}
		}
	}
	// If it's odd then it's inside of the polygon, otherwise it's outside of the polygon.
	return count%2 != 0
}
