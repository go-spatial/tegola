package subject

import (
	"fmt"

	"github.com/terranodo/tegola/container/singlelist/point/list"
	"github.com/terranodo/tegola/maths"
)

type Pair struct {
	pts [2]*list.Pt
	l   *list.List
}

func (p *Pair) Next() *Pair {
	if p == nil {
		return nil
	}
	pt1 := p.pts[1]
	pt2e := pt1.Next()
RetryPt2E:
	if pt2e == p.l.Front() {
		return nil
	}
	pt2, ok := pt2e.(*list.Pt)
	if !ok {
		pt2e = pt2e.Next()
		goto RetryPt2E
	}
	return &Pair{
		pts: [2]*list.Pt{pt1, pt2},
		l:   p.l,
	}
}

func (p *Pair) Pt1() *list.Pt      { return p.pts[0] }
func (p *Pair) Pt2() *list.Pt      { return p.pts[1] }
func (p *Pair) AsLine() maths.Line { return maths.Line{p.pts[0].Point(), p.pts[1].Point()} }
func (p *Pair) PushInBetween(e list.ElementerPointer) bool {
	return p.l.PushInBetween(p.pts[0], p.pts[1], e)
}

func (p *Pair) GoString() string {
	return fmt.Sprintf("p[%p]( %v %v )", p, p.Pt1().Pt, p.Pt2().Pt)
}
