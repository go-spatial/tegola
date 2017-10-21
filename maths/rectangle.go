package maths

type Rectangle [2][2]float64

func (r Rectangle) Contains(pt Pt) bool {
	return pt.X >= r[0][0] && pt.Y >= r[0][1] && pt.X <= r[1][0] && pt.Y <= r[1][1]
}
