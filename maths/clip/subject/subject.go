package subject

import (
	"errors"

	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

// ErrInvalidCoordsNumber is the error produced when the number of coordinates provided is not even or large enough to from a linestring.
var ErrInvalidCoordsNumber = errors.New("Event number of coords expected.")

type Subject struct {
	winding maths.WindingOrder
	list.List
}

func New(coords []float64) (*Subject, error) {
	return new(Subject).Init(coords)
}

func (s *Subject) Init(coords []float64) (*Subject, error) {
	if len(coords)%2 != 0 {
		return nil, ErrInvalidCoordsNumber
	}
	s.winding = maths.WindingOrderOf(coords)
	s.List.Init()

	for x, y := 0, 1; x < len(coords); x, y = x+2, y+2 {
		s.PushBack(list.NewPoint(coords[x], coords[y]))
	}
	return s, nil
}

func (s *Subject) Winding() maths.WindingOrder { return s.winding }

func (s *Subject) FirstPair() *Pair {
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
	return &Pair{
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
