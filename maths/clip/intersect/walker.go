package intersect

import (
	"github.com/terranodo/tegola/container/list/point/list"

	"github.com/terranodo/tegola/maths"
)

type Inbound struct {
	pt *Point
}

func NewInbound(pt *Point) *Inbound {
	if pt == nil {
		return nil
	}
	return &Inbound{pt: pt}
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
			return NewInbound(pt)
		}
	}
	return nil
}

func (ib *Inbound) Walk(fn func(idx int, pt maths.Pt) bool) {
	fib := ib.pt
	if !fn(0, fib.Point()) {
		return
	}
	var pt maths.Pt
	var ipt *Point
	seen := make(map[list.Elementer]bool)
	for p, i := fib.NextWalk(), 1; p != nil; i++ {
		op := p
		if seen[p] {
			//log.Printf("Already saw %p -- cycle bailing.\n", p)
			return
		}
		seen[p] = true
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
		if ipt == fib {
			return
		}
		if ipt != nil {
			if ipt.Inward {
				ib.pt = ipt
			}
			pt = ipt.Point()
			p = ipt.NextWalk()
		}
		if fib.Point().IsEqual(pt) {
			//log.Println("fib point value is same.")
			return
		}

		if p == nil {
			p = op.List().Front()
		}
		//log.Printf("Looking Point(%v) looking at pt(%p)%[2]v fib(%p)%[3]v\n", i, p, fib)

		if !fn(i, pt) {
			return
		}
	}
}
