package maths

import "math"

type Line [2]Pt

func NewLine(x1, y1, x2, y2 float64) Line {
	return Line{
		Pt{x1, y1},
		Pt{x2, y2},
	}
}

// InBetween will check to see if the given point lies the line provided inbetween the endpoints.
func (l Line) InBetween(pt Pt) bool {
	lx, gx := l[0].X, l[1].X
	if l[0].X > l[1].X {
		lx, gx = l[1].X, l[0].X
	}
	ly, gy := l[0].Y, l[1].Y
	if l[0].Y > l[1].Y {
		ly, gy = l[1].Y, l[0].Y
	}
	return lx <= pt.X && pt.X <= gx && ly <= pt.Y && pt.Y <= gy

}
func (l Line) ExInBetween(pt Pt) bool {
	lx, gx := l[0].X, l[1].X
	if l[0].X > l[1].X {
		lx, gx = l[1].X, l[0].X
	}
	ly, gy := l[0].Y, l[1].Y
	if l[0].Y > l[1].Y {
		ly, gy = l[1].Y, l[0].Y
	}

	goodx, goody := lx < pt.X && pt.X < gx, ly < pt.Y && pt.Y < gy
	if gx-lx == 0 {
		goodx = true
	}
	if gy-ly == 0 {
		goody = true
	}

	//log.Println(l, pt, ":", lx, "<", pt.X, "&&", pt.X, "<", gx, "&&", ly, "<", pt.Y, "&&", pt.Y, "<", gy, goodx, goody)
	return goodx && goody

}

func (l Line) IsVertical() bool {
	return l[0].X == l[1].X
}
func (l Line) IsHorizontal() bool {
	return l[0].Y == l[1].Y
}

//Clamp will return a point that is on the line based on pt. It will do this by restricting each of the coordiantes to the line.
func (l Line) Clamp(pt Pt) (p Pt) {
	p = pt
	lx, gx := l[0].X, l[1].X
	if l[0].X > l[1].X {
		lx, gx = l[1].X, l[0].X
	}
	ly, gy := l[0].Y, l[1].Y
	if l[0].Y > l[1].Y {
		ly, gy = l[1].Y, l[0].Y
	}

	if pt.X < lx {
		p.X = lx
	}
	if pt.X > gx {
		p.X = gx
	}
	if pt.Y < ly {
		p.Y = ly
	}
	if pt.Y > gy {
		p.Y = gy
	}
	return p
}

//InsideEx will return true if the given point is consider to be on the inside of the line, for the given winding order. The inside region excludes the line.
// For clockwise the inside edge for a line heading in the upward direction is any point to the right of the line.
// For clockwise the inside edge for a line heading in the downward direction is any point to the left of the line.
// For clockwise the inside edge for a line heading in the rightward direction is any point to the bottom of the line.
// For clockwise the inside edge for a line heading in the leftward direction is any point above the line.
/*
func (l Line) InsideEx(w WindingOrder, pt  Pt) bool {
insideX, insideY := true, true
clockwise = w.IsClockwise()
	deltaX := l[1].X - l[0].X
	deltaY := l[1].Y - l[0].Y

	// Dealing with a horizontal line.
	if deltaX == 0 {
	     ptDelta := l[0].X - pt.X
	     if clockwise {

	     }
	}



	switch {
	case w.IsClockwise() && deltaX > 0: // right ward.
		goody =

	}

}
*/
// DistanceFromPoint will return the perpendicular distance from the point.
func (l Line) DistanceFromPoint(pt Pt) float64 {

	deltaX := l[1].X - l[0].X
	deltaY := l[1].Y - l[0].Y
	//log.Println("delta X/Y :  pt - line", deltaX, deltaY, pt, l)
	denom := math.Abs((deltaY * pt.X) - (deltaX * pt.Y) + (l[1].X * l[0].Y) - (l[1].Y * l[0].X))
	num := math.Sqrt(math.Pow(deltaY, 2) + math.Pow(deltaX, 2))
	//log.Println("denim/num", denom, num)
	if num == 0 {
		return 0
	}
	return denom / num
}

// SlopeIntercept will find the slop (if there is one) and the intercept of the two provided lines. If there isn't a slope because the lines are verticle, the slopeDefined will be false.
func (l Line) SlopeIntercept() (m, b float64, defined bool) {
	dx := l[1].X - l[0].X
	dy := l[1].Y - l[0].Y
	if dx == 0 || dy == 0 {
		// if dx == 0 then m == 0; and the intercept is y.
		// However if the lines are verticle then the slope is not defined.
		return 0, l[0].Y, dx != 0
	}
	m = dy / dx
	// b = y - mx
	b = l[0].Y - (m * l[0].X)
	return m, b, true
}

// DeltaX returns the difference between the x coordinates of point2 and point1.
func (l Line) DeltaX() float64 { return l[1].X - l[0].X }

// DeltaY returns the difference between the y coordinates of point2 and point1.
func (l Line) DeltaY() float64 { return l[1].Y - l[0].Y }

// IsLeft  tests if point P2 is Left|On|Right of the line P0 to P1.
//      returns: >0 for left, 0 for on, and <0 for  right of the line.
func (l Line) IsLeft(pt Pt) float64 {
	return (l.DeltaX() * (pt.Y - l[0].Y)) - ((pt.X - l[0].X) * l.DeltaY())
}

// LeftRightMostPt returns the left and right most points of the vertexes of the line.
func (l Line) LeftRightMostPts() (Pt, Pt) {
	if XYOrder(l[0], l[1]) < 0 {
		return l[0], l[1]
	}
	return l[1], l[0]
}

// LeftRightMostAsLine returns the left most and right most points as a line going from the left to the right.
func (l1 Line) LeftRightMostAsLine() Line {
	l,r := l1.LeftRightMostPts()
	return Line{l,r}
}
