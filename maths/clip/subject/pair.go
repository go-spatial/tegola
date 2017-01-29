package subject

import (
	"fmt"

	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

type pair struct {
	pts [2]*list.Pt
	l   *list.List
}

func (p *pair) Next() *pair {
	if p == nil {
		return nil
	}
	pt1 := p.pts[1]
	pt2e := pt1.Next()
	if pt2e == nil {
		return nil
	}
	if pt2, ok := pt2e.(*list.Pt); !ok {
		return nil
	} else {
		p.pts[0], p.pts[1] = pt1, pt2
	}
	return p
}

func (p *pair) Pt1() *list.Pt      { return p.pts[0] }
func (p *pair) Pt2() *list.Pt      { return p.pts[1] }
func (p *pair) AsLine() maths.Line { return maths.Line{p.pts[0].Point(), p.pts[1].Point()} }
func (p *pair) PushInBetween(e list.ElementerPointer) bool {
	return p.l.PushInBetween(p.pts[0], p.pts[1], e)
}

func (p *pair) GoString() string {
	return fmt.Sprintf("p[%p]( %#v %#v )", p, p.Pt1(), p.Pt2())
}
