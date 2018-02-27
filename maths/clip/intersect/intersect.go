package intersect

import (
	"github.com/go-spatial/tegola/container/singlelist/point/list"
)

type Intersect struct {
	list.List
}

func New() *Intersect {
	l := new(Intersect)

	return l
}

func (i *Intersect) FirstInboundPtWalker() *Inbound {
	if i == nil || i.Len() == 0 {
		return nil
	}
	// We have to find the first Inbound Pt.
	var ok bool
	var pt *Point
	for p := i.Front(); ; p = p.Next() {
		if pt, ok = p.(*Point); ok && pt.Inward {
			break
		}
		if p == i.Back() {
			// did not find a point.
			return nil
		}
	}
	return NewInbound(pt)
}

func (i *Intersect) ForEach(fn func(*Point) bool) {
	i.List.ForEach(func(e list.ElementerPointer) bool {
		pt, ok := e.(*Point)
		if !ok {
			// Skip things that are not Intersect points.
			return true
		}
		return fn(pt)
	})
}

/*
func (i *Intersect) ResetSeen() {
	i.ForEach(func(pt *Point) bool {
		pt.Seen = false
		return true
	})
}
*/
