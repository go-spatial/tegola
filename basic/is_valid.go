package basic

import (
	"log"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths"
)

// IsValid returns whether the line is valid according to the OGC specifiction
// The line should not intersect it's self.
func (l Line) IsValid() bool {
	// First let's run through all the points and see if any of them are
	// repeated.
	var seen map[string]struct{}
	for _, pt := range l {
		// The map contains the point. Which means the point is duplicated.
		if _, ok := seen[pt.String()]; ok {
			//log.Println("Saw point:", pt)
			return false
		}
	}
	// We need to loop through the pair of points to see if they intersect.
	//log.Printf("Line: %#v", l)
	pt0 := l[len(l)-1]
	endj := len(l) - 1
	for i, pt1 := range l[:len(l)-2] {
		inpt0 := l[i+1]
		// should the end of the range be len(l) for everything other then
		// the first time?
		for _, inpt1 := range l[i+2 : endj] {
			// If we are looking at the same point
			//log.Println(i, j, "Looking:", pt0, pt1, inpt0, inpt1)
			if tegola.IsPointEqual(pt0, inpt0) && tegola.IsPointEqual(pt1, inpt1) {
				continue
			}
			l1, l2 := maths.Line{pt0.AsPt(), pt1.AsPt()}, maths.Line{inpt0.AsPt(), inpt1.AsPt()}
			if ipt, ok := maths.Intersect(l1, l2); ok {
				if l1.InBetween(ipt) && l2.InBetween(ipt) {
					//log.Println("The lines intersect:", l1, l2, ipt)
					return false
				}
			}
			inpt0 = inpt1
		}
		pt0 = pt1
		endj = len(l)
	}
	return true
}

// IsValid returns weather the polygon is valid according to the OGC specifiction.
func (p Polygon) IsValid() bool {
	// If there are not linestrings in a polygon then it is invalid.
	if len(p) == 0 {
		return false
	}
	/*
	   A Polygon is valid if the first linestring is clockwise and
	*/
	if !(p[0].IsValid() && p[0].Direction() == maths.Clockwise) {
		log.Println("Line 0", p[0].IsValid(), p[0].Direction())
		return false
	}
	/*
	   all other linestrings are counter-clockwise and contained by
	   the first linestring.
	*/
	for _, l := range p[1:] {
		if !(l.IsValid() && l.Direction() == maths.CounterClockwise && p[0].ContainsLine(l)) {
			return false
		}
	}
	return true
}
