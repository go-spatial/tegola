package intersect

import (
	"github.com/terranodo/tegola/container/list/point/list"

	"github.com/terranodo/tegola/maths"
)

type Inbound struct {
	pt   *Point
	seen map[list.Elementer]bool
}

func NewInbound(pt *Point) *Inbound {
	if pt == nil {
		return nil
	}
	seen := make(map[list.Elementer]bool)
	return &Inbound{pt: pt, seen: seen}
}

func (ib *Inbound) Next() (nib *Inbound) {
	var pt *Point
	var ok bool
	for p := ib.pt.Next(); p != nil; p = p.Next() {
		pt, ok = p.(*Point)
		if !ok {
			pt = nil
			continue
		}
		if pt.Inward {
			nib := NewInbound(pt)
			nib.seen = ib.seen
			return nib
		}
	}
	return nil
}

func (ib *Inbound) Walk(fn func(idx int, pt maths.Pt) bool) {

	firstInboundPoint := ib.pt
	icount := 0
	if !fn(0, firstInboundPoint.Point()) {
		// log.Printf("Bailing after first point.")
		return
	}
	var pt maths.Pt
	var ipt *Point
	for i, p := 1, firstInboundPoint.NextWalk(); p != nil; i++ {
		op := p
		//log.Printf("Walk Looking at %#v", p)
		if ib.seen[p] {
			//log.Printf("Already saw %p -- cycle bailing.\n", p)
			return
		}
		ib.seen[p] = true
		switch ppt := p.(type) {
		case *Point:
			ipt = ppt
		case *SubjectPoint:
			ipt = ppt.AsIntersectPoint()
		case *RegionPoint:
			ipt = ppt.AsIntersectPoint()
		case list.ElementerPointer:
			ipt = nil
			pt = ppt.Point()
			p = ppt.Next()
		default:
			continue
		}
		if ipt == firstInboundPoint {
			return
		}
		if ipt != nil {
			if ipt.Inward {
				ib.pt = ipt
			}
			pt = ipt.Point()
			p = ipt.NextWalk()
		}
		if firstInboundPoint.Point().IsEqual(pt) {
			icount++
			// icount of 3 because, if we see the same point 3 times, there is an issue.
			// 1 time makes sense.
			// 2 time if the point is on the border.
			// 3 times if a point from the outside goes to the border point back to a point outside.
			// 4+ we have an issue.
			if icount == 3 {
				// log.Println("firstInboundPoint point value is same.")
				return
			}
		}

		if p == nil {
			p = op.List().Front()
		}
		//log.Printf("Looking Point(%v) looking at pt(%p)%[2]v firstInboundPoint(%p)%[3]v\n", i, p, firstInboundPoint)

		if !fn(i, pt) {
			return
		}
	}
}
