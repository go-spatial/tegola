package maths

import "github.com/terranodo/tegola"

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

func WindingOrderOf(sub []float64) WindingOrder {
	sum := 0
	for x, y := 0, 1; y < len(sub); x, y = x+2, y+2 {
		nx, ny := x+2, y+2
		if y == (len(sub) - 1) {
			nx, ny = 0, 1
		}
		sum += int((sub[nx] - sub[x]) * (sub[ny] + sub[y]))
	}
	if sum < 0 {
		return Clockwise
	}
	return CounterClockwise
}

func WindingOrderOfLine(l tegola.LineString) WindingOrder {
	sum := 0
	pts := l.Subpoints()
	for i, pt := range pts {
		ni := i + 1
		if ni == len(pts) {
			ni = 0
		}
		npt := pts[ni]
		sum += int((npt.X() - pt.X()) * (npt.Y() + pt.Y()))
	}
	if sum < 0 {
		return Clockwise
	}
	return CounterClockwise
}
