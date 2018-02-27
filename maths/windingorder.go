package maths

import (
	"github.com/go-spatial/tegola"
)

// WindingOrder the direction the line strings.
type WindingOrder uint8

const (
	Clockwise WindingOrder = iota
	CounterClockwise
)

func (w WindingOrder) String() string {
	switch w {
	case Clockwise:
		return "clockwise"
	case CounterClockwise:
		return "counter clockwise"
	}
	return "unknown"
}

func (w WindingOrder) IsClockwise() bool        { return w == Clockwise }
func (w WindingOrder) IsCounterClockwise() bool { return w == CounterClockwise }
func (w WindingOrder) Not() WindingOrder {
	if w == Clockwise {
		return CounterClockwise
	}
	return Clockwise
}

func WindingOrderOfPts(pts []Pt) WindingOrder {
	sum := 0.0
	li := len(pts) - 1

	for i := range pts[:li] {
		sum += (pts[i].X * pts[i+1].Y) - (pts[i+1].X * pts[i].Y)
	}
	sum += (pts[li].X * pts[0].Y) - (pts[0].X * pts[li].Y)

	//log.Println("For pts:", pts, "sum", sum)

	if sum < 0 {
		return CounterClockwise
	}
	return Clockwise
}

func WindingOrderOf(sub []float64) WindingOrder {
	pts := make([]Pt, 0, len(sub)/2)
	for x, y := 0, 1; y < len(sub); x, y = x+2, y+2 {
		pts = append(pts, Pt{sub[x], sub[y]})
	}
	return WindingOrderOfPts(pts)
}

func WindingOrderOfLine(l tegola.LineString) WindingOrder {
	lpts := l.Subpoints()
	pts := make([]Pt, 0, len(lpts))
	for _, pt := range lpts {
		pts = append(pts, Pt{pt.X(), pt.Y()})
	}
	return WindingOrderOfPts(pts)
}
