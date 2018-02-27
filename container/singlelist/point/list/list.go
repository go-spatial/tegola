package list

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-spatial/tegola/container/singlelist"
	"github.com/go-spatial/tegola/maths"
)

type List struct {
	list.List
}

// ForEachIdx is the same as the one in singlelist except it takes ElementerPointers
func (l *List) ForEachIdx(fn func(int, ElementerPointer) bool) {
	l.List.ForEachIdx(func(idx int, e list.Elementer) bool {
		el, ok := e.(ElementerPointer)
		if !ok {
			return true
		}
		return fn(idx, el)
	})
}

// ForEachIdx is the same as the one in singlelist except it takes ElementerPointers
func (l *List) ForEach(fn func(ElementerPointer) bool) {
	l.List.ForEach(func(e list.Elementer) bool {
		el, ok := e.(ElementerPointer)
		if !ok {
			return true
		}
		return fn(el)
	})
}

// ForEachPt will iterate forward through the list. The fn will be called for each pt.
// If fn returns false, the iteration will stop.
func (l *List) ForEachPt(fn func(int, maths.Pt) bool) {
	l.ForEachIdx(func(idx int, pt ElementerPointer) bool {
		return fn(idx, pt.Point())
	})
}

// ForEachPtBetween will iterate forward through the list. Starting from the start point to the end point calling the fn.
func (l *List) ForEachPtBetween(start, end ElementerPointer, fn func(int, maths.Pt) bool) {
	count := 0
	l.FindElementsBetween(start, end, func(e list.Elementer) bool {
		pt, ok := e.(maths.Pointer)
		count++
		if !ok {
			// skip elements in the list that are not pointers.
			return false
		}

		return !fn(count-1, pt.Point())
	})
}

// PushInBetween will Push the element pointer between the start and end points
func (l *List) PushInBetween(start, end ElementerPointer, element ElementerPointer) (r bool) {
	spt := start.Point().Truncate()
	ept := end.Point().Truncate()
	var mark ElementerPointer

	defer func() {
		if r && element.Next() == nil {
			log.Println("nil!")
			log.Printf("\tstart: %v[%[1]p] %v[%[2]p]", start, start.Next())
			log.Printf("\t   pt: %v[%[1]p] %v[%[2]p]", element, element.Next())
			log.Printf("\t  end: %v[%[1]p] %v[%[2]p]", end, end.Next())
			//log.Printf("\t mark: %v[%[1]p] %v[%[2]p]", mark, mark.Next())
			log.Printf("\t mark: %v[%[1]p] ]", mark)
			panic("Stop!")
		}
	}()

	mpt := element.Point().Truncate()
	{
		line := maths.Line{spt, ept}
		// Make sure the point is in between the starting and ending point.
		if !line.InBetween(mpt) {
			return false
		}
	}

	// Need to figure out if points are increasing or decreasing in the x direction.
	deltaX := ept.X - spt.X
	deltaY := ept.Y - spt.Y
	xIncreasing := deltaX > 0
	yIncreasing := deltaY > 0

	// fmt.Printf("// start:  mpt,ept %f,%f,%f,%f,%v\n", mpt.X, ept.X, mpt.Y, ept.Y, mpt.X == ept.X && mpt.Y == ept.Y)

	// If it's equal to the end point, we will push it before the end point
	if ept.IsEqual(mpt) {
		l.InsertBefore(element, end)
		return true
	}

	// If it's equal to the start point we need to push it after the start.
	if spt.IsEqual(mpt) {
		l.InsertAfter(element, start)
		return true
	}

	fmark := l.FindElementsBetween(start, end, func(e list.Elementer) bool {
		var goodX, goodY = true, true
		if ele, ok := e.(maths.Pointer); ok {
			pt := ele.Point()

			// There is not change in X when deltaX == 0; so it's always good.
			if deltaX != 0 {
				if xIncreasing {
					goodX = int64(mpt.X) <= int64(pt.X)
				} else {
					goodX = int64(mpt.X) >= int64(pt.X)
				}
			}
			// There is no change in Y when deltaY == 0; so it's always good.
			if deltaY != 0 {
				if yIncreasing {
					goodY = int64(mpt.Y) <= int64(pt.Y)
				} else {
					goodY = int64(mpt.Y) >= int64(pt.Y)
				}
			}
			return goodX && goodY
		}

		return false
	})
	mark, _ = fmark.(ElementerPointer)

	// This should not happen?
	if mark == nil {
		l.InsertBefore(element, end)
		return true
	}

	l.InsertBefore(element, mark)

	return true
}

func (l *List) GoString() string {
	if l == nil || l.Len() == 0 {
		return "List{}"
	}
	strs := []string{"List{"}
	l.ForEach(func(pt ElementerPointer) bool {
		strs = append(strs, fmt.Sprintf("%v(%p:%[2]T)", pt.Point(), pt))
		return true
	})
	strs = append(strs, "}")
	return strings.Join(strs, "")
}

func New() *List {
	return &List{
		List: *list.New(),
	}
}
