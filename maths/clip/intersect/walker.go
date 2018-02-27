package intersect

import (
	"github.com/go-spatial/tegola/container/singlelist/point/list"
	"github.com/go-spatial/tegola/maths"
)

type Inbound struct {
	pt    *Point
	seen  map[list.Elementer]bool
	iseen map[*Point]bool
}

func NewInbound(pt *Point) *Inbound {
	if pt == nil {
		return nil
	}
	seen := make(map[list.Elementer]bool)
	iseen := make(map[*Point]bool)
	return &Inbound{pt: pt, seen: seen, iseen: iseen}
}

func (ib *Inbound) Next() (nib *Inbound) {
	var pt *Point
	var ok bool

	for p := ib.pt.Next(); ib.pt != p; p = p.Next() {
		if p == nil {
			return
		}
		// skip elements that are not points.
		if pt, ok = p.(*Point); !ok {
			continue
		}
		ipp := asIntersect(p)
		if pt.Inward && !ib.iseen[ipp] {
			nib := NewInbound(pt)
			nib.seen = ib.seen
			nib.iseen = ib.iseen
			return nib
		}
	}
	return nil
}

func next(p list.Elementer) list.Elementer {
	switch ppt := p.(type) {
	case *Point:
		return ppt.NextWalk()
	case *SubjectPoint:
		ipt := ppt.AsIntersectPoint()
		return ipt.NextWalk()
	case *RegionPoint:
		ipt := ppt.AsIntersectPoint()
		return ipt.NextWalk()
	case list.ElementerPointer:
		return ppt.Next()
	default:
		return nil
	}
}

func asIntersect(p list.Elementer) *Point {
	switch ppt := p.(type) {
	case *Point:
		return ppt
	case *SubjectPoint:
		return ppt.AsIntersectPoint()
	case *RegionPoint:
		return ppt.AsIntersectPoint()

	default:
		return nil
	}
}

func (ib *Inbound) Walk(fn func(idx int, pt maths.Pt) bool) {

	firstInboundPoint := ib.pt
	if ib.iseen[firstInboundPoint] {
		// we already saw this point, let's move on.
		//log.Printf("Already saw (%.3v) at (%p)%[2]#v  upping count(%v).", 0, firstInboundPoint, 0)
		return
	}

	//log.Printf("Walk Looking at %#v", firstInboundPoint)
	if !fn(0, firstInboundPoint.Point()) {
		// log.Printf("Bailing after first point.")
		return
	}

	ib.seen[firstInboundPoint] = true
	ib.iseen[firstInboundPoint] = true
	//count := 0
	//log.Println("\n\nStarting walk:\n\n")
	//defer log.Println("\nEnding Walk\n")
	for i, p := 1, next(firstInboundPoint); ; i, p = i+1, next(p) {
		// We have found the original point.

		ipp := asIntersect(p)
		if ipp == firstInboundPoint {
			//log.Println("Back to the begining.\n\n\n")
			return
		}

		//log.Printf("(%.3v)Walk Looking at %#v", i, p)
		if ib.seen[p] {
			/*
				count++
				log.Printf("Already saw (%.3v) at (%p)%[2]#v  upping count(%v).", i, p, count)
				if count > 10 {
			*/
			return
			//			}
		}

		ib.seen[p] = true
		if ipp != nil {
			ib.iseen[ipp] = true
		}

		pter, ok := p.(list.ElementerPointer)
		if !ok {
			// skip entries that are not points.
			continue
		}

		//log.Printf("Looking at Point(%.3v) looking at pt(%p)%[2]v", i, p)
		if !fn(i, pter.Point()) {
			return
		}
	}
}
