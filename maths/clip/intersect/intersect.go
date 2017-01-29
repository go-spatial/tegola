package intersect

import "github.com/terranodo/tegola/container/list/point/list"

type Intersect struct {
	list.List
}

func New() *Intersect {
	l := new(Intersect)
	l.List.Init()
	return l
}

func (i *Intersect) FirstInboundPtWalker() *Inbound {
	if i == nil || i.Len() == 0 {
		return nil
	}
	// We have to find the first Inbound Pt.
	var ok bool
	var pt *Point
	for p := i.Front(); p != nil; p = p.Next() {
		if pt, ok = p.(*Point); ok && pt.Inward {
			break
		}
	}
	return NewInbound(pt)
}
