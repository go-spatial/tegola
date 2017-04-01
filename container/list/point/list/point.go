package list

import (
	"fmt"
	"strings"

	"github.com/terranodo/tegola/container/list"
	"github.com/terranodo/tegola/maths"
)

type Elementer interface {
	list.Elementer
}

type ElementerPointer interface {
	list.Elementer
	maths.Pointer
}

type Pt struct {
	maths.Pt
	list.Sentinel
}

func (p *Pt) Point() (pt maths.Pt) { return p.Pt }
func (p *Pt) String() string {
	if p == nil {
		return "(nil)"
	}
	return p.Pt.String()
}
func (p *Pt) GoString() string {
	if p == nil {
		return "(nil)"
	}
	return fmt.Sprintf("[%v,%v]", p.Pt.X, p.Pt.Y)
}

func NewPt(pt maths.Pt) *Pt {
	return &Pt{Pt: pt}
}
func NewPoint(x, y float64) *Pt {
	return &Pt{
		Pt: maths.Pt{
			X: x,
			Y: y,
		},
	}
}

func NewPointSlice(pts ...maths.Pt) (rpts []*Pt) {
	for _, pt := range pts {
		rpts = append(rpts, &Pt{Pt: pt})
	}
	return rpts
}

type List struct {
	list.List
}

// ForEachPt will iteratate forward through the list, call the fn for each pt.
// If fn returns false, the iteration will stop.
func (l *List) ForEachPt(fn func(idx int, pt maths.Pt) (cont bool)) {
	for i, p := 0, l.Front(); p != nil; i, p = i+1, p.Next() {
		pt := p.(maths.Pointer).Point()
		if !fn(i, pt) {
			break
		}
	}
}

func (l *List) PushInBetween(start, end ElementerPointer, element ElementerPointer) bool {
	spt := start.Point()
	ept := end.Point()
	mpt := element.Point()

	// Need to figure out if points are increasing or decreasing in the x direction.
	deltaX := ept.X - spt.X
	deltaY := ept.Y - spt.Y
	xIncreasing := deltaX > 0
	yIncreasing := deltaY > 0

	// If it's equal to the end point, we will push it after the end point
	if mpt.X == ept.X && mpt.Y == ept.Y {
		l.InsertAfter(element, end)
		return true
	}

	// Need to check that the point equal to or ahead of the start point
	if deltaX != 0 {
		if xIncreasing {
			if mpt.X < spt.X {
				return false
			}
		} else {
			if mpt.X > spt.X {
				return false
			}
		}
	}
	// There is no change in Y when deltaY == 0; so it's always good.
	if deltaY != 0 {
		if yIncreasing {
			if mpt.Y < spt.Y {
				return false
			}
		} else {
			if mpt.Y > spt.Y {
				return false
			}
		}
	}

	// Need to check that the point equal to or behind of the end point
	if deltaX != 0 {
		if xIncreasing {
			if mpt.X > ept.X {
				return false
			}
		} else {
			if mpt.X < ept.X {
				return false
			}
		}
	}
	// There is no change in Y when deltaY == 0; so it's always good.
	if deltaY != 0 {
		if yIncreasing {
			if mpt.Y > ept.Y {
				return false
			}
		} else {
			if mpt.Y < ept.Y {
				return false
			}
		}
	}

	mark := l.FindElementForward(start.Next(), end, func(e list.Elementer) bool {
		var goodX, goodY = true, true
		if ele, ok := e.(maths.Pointer); ok {
			pt := ele.Point()

			// There is not change in X when deltaX == 0; so it's always good.
			if deltaX != 0 {
				if xIncreasing {
					goodX = mpt.X < pt.X
				} else {
					goodX = mpt.X > pt.X
				}
			}
			// There is no change in Y when deltaY == 0; so it's always good.
			if deltaY != 0 {
				if yIncreasing {
					goodY = mpt.Y < pt.Y
				} else {
					goodY = mpt.Y > pt.Y
				}
			}
			return goodX && goodY
		}
		return false
	})
	if mark == nil {
		// check to see if the point is equal to start.
		if mpt.X == spt.X && mpt.Y == spt.Y {
			l.InsertAfter(element, start)
			return true
		}
		return false
	}

	l.InsertBefore(element, mark)
	return true
}

func (l *List) GoString() string {
	if l == nil || l.Len() == 0 {
		return "List{}"
	}
	strs := []string{"List{"}
	for p := l.Front(); p != nil; p = p.Next() {
		pt := p.(maths.Pointer)
		strs = append(strs, fmt.Sprintf("%v(%p:%[2]T)", pt.Point(), p))
	}
	strs = append(strs, "}")
	return strings.Join(strs, "")
}

func New() *List {
	return &List{
		List: *list.New(),
	}
}
