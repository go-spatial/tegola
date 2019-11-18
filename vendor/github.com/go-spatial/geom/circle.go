package geom

import (
	"errors"
	"math"
)

// ErrPointsAreCoLinear is thrown when points are colinear but that is unexpected
var ErrPointsAreCoLinear = errors.New("given points are colinear")

// Circle is a point (float tuple) and a radius
type Circle struct {
	Center [2]float64
	Radius float64
}

// IsColinear returns weather the a,b,c are colinear to each other
func IsColinear(a, b, c [2]float64) bool {
	xA, yA, xB, yB, xC, yC := a[0], a[1], b[0], b[1], c[0], c[1]
	return ((yB - yA) * (xC - xB)) == ((yC - yB) * (xB - xA))
}

// CircleFromPoints returns the circle from by the given points, or an error if the points are colinear.
// REF:  Formula used gotten from http://mathforum.org/library/drmath/view/55233.html
func CircleFromPoints(a, b, c [2]float64) (Circle, error) {
	xA, yA, xB, yB, xC, yC := a[0], a[1], b[0], b[1], c[0], c[1]
	if ((yB - yA) * (xC - xB)) == ((yC - yB) * (xB - xA)) {
		return Circle{}, ErrPointsAreCoLinear
	}

	xDeltaA, xDeltaB := xB-xA, xC-xB

	// Rotate the points if inital set is not ideal
	// This will terminate as we have already done the Colinear check above.
	for xDeltaA == 0 || xDeltaB == 0 {
		xA, yA, xB, yB, xC, yC = xB, yB, xC, yC, xA, yA
		xDeltaA, xDeltaB = xB-xA, xC-xB
	}

	yDeltaA, yDeltaB := yB-yA, yC-yB

	midAB := [2]float64{(xA + xB) / 2, (yA + yB) / 2}
	midBC := [2]float64{(xB + xC) / 2, (yB + yC) / 2}

	var x, y float64

	switch {
	case yDeltaA == 0 && xDeltaB == 0: // slopeA && slopeB == ∞
		x, y = midAB[0], midBC[1]

	case yDeltaA == 0 && xDeltaB != 0:
		slopeB := yDeltaB / xDeltaB
		x = midAB[0]
		y = midBC[1] + ((midBC[0] - x) / slopeB)

	case yDeltaB == 0 && xDeltaA == 0:
		x, y = midBC[0], midAB[1]

	case yDeltaB == 0 && xDeltaA != 0:
		slopeA := yDeltaA / xDeltaA
		x = midBC[0]
		y = midAB[1] + (midAB[0]-x)/slopeA

	case xDeltaA == 0:
		slopeB := yDeltaB / xDeltaB
		y = midBC[1]
		x = slopeB*(midBC[1]-y) + midBC[0]

	case xDeltaB == 0:
		slopeA := yDeltaA / xDeltaA
		y = midBC[1]
		x = slopeA*(midAB[1]-y) + midAB[0]
	default:
		slopeA := yDeltaA / xDeltaA
		slopeB := yDeltaB / xDeltaB

		x = (((slopeA * slopeB * (yA - yC)) +
			(slopeB * (xA + xB)) -
			(slopeA * (xB + xC))) /
			(2 * (slopeB - slopeA)))
		y = (-1/slopeA)*(x-(xA+xB)*0.5) + ((yA + yB) * 0.5)
	}

	// get the correct slopes

	vA, vB := x-xA, y-yA
	r := math.Sqrt((vA * vA) + (vB * vB))
	return Circle{
		Center: [2]float64{x, y},
		Radius: RoundToPrec(r, 4),
	}, nil
}

// ContainsPoint will check to see if the point is in the circle.
func (c Circle) ContainsPoint(pt [2]float64) bool {
	// get the distance between the center and the point, and if it's greater then the radius it's outside
	// of the circle.
	v1, v2 := c.Center[0]-pt[0], c.Center[1]-pt[1]
	d := math.Sqrt((v1 * v1) + (v2 * v2))
	d = RoundToPrec(d, 3)
	return c.Radius >= d
}

func (c Circle) AsPoints(k uint) []Point {
	if k < 3 {
		k = 30
	}

	pts := make([]Point, int(k))
	for i := 0; i < int(k); i++ {
		t := (2 * math.Pi) * (float64(i) / float64(k))
		x, y := c.Center[0]+c.Radius*math.Cos(t), c.Center[1]+c.Radius*math.Sin(t)
		pts[i][0], pts[i][1] = float64(x), float64(y)
	}
	return pts
}

func (c Circle) AsLineString(k uint) LineString {
	pts := c.AsPoints(k)
	lns := make(LineString, len(pts))
	for i := range pts {
		lns[i] = [2]float64(pts[i])
	}
	return lns
}

// AsSegments takes the number of segments that should be returned to describe the circle.
// a value  less then 3 will use the default value of 30.
func (c Circle) AsSegments(k uint) []Line {
	pts := c.AsPoints(k)
	lines := make([]Line, len(pts))
	for i := range pts {
		j := i - 1
		if j < 0 {
			j = int(k) - 1
		}
		lines[i][0] = pts[j]
		lines[i][1] = pts[i]
	}
	return lines
}
