package subject

import "github.com/terranodo/tegola/container/list/point/list"

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

func (p *pair) Pt1() *list.Pt { return p.pts[0] }
func (p *pair) Pt2() *list.Pt { return p.pts[1] }
