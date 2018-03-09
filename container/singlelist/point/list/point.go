package list

import (
	"fmt"

	"github.com/go-spatial/tegola/container/singlelist"
	"github.com/go-spatial/tegola/maths"
)

type Elementer interface {
	list.Elementer
}

type ElementerPointer interface {
	list.Elementer
	maths.Pointer
}

// Pt is a point node.
type Pt struct {
	maths.Pt
	// This fulfills the Elementer interface.
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
