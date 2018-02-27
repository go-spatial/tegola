package plyg

import (
	"sort"

	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/points"
)

type Builder struct {
	parts [2][]maths.Pt
	label maths.Label
}

type YPart struct {
	Y   float64
	Idx int
}

type YPartByY []YPart

func (ys YPartByY) Len() int           { return len(ys) }
func (ys YPartByY) Swap(i, j int)      { ys[i], ys[j] = ys[j], ys[i] }
func (ys YPartByY) Less(i, j int) bool { return ys[i].Y < ys[j].Y }

func (b *Builder) CurrentRing() (ring Ring, x1 float64, y1s []YPart, x2 float64, y2s []YPart) {
	if b == nil {
		return ring, x1, y1s, x2, y2s
	}
	ring.Label = b.label
	if len(b.parts[0]) == 0 || len(b.parts[1]) == 0 {
		return ring, x1, y1s, x2, y2s
	}

	x1 = b.parts[0][0].X
	x2 = b.parts[1][0].X

	ring.Points = make([]maths.Pt, 0, len(b.parts[0])+len(b.parts[1]))
	var pts []maths.Pt
	pts = append(pts, b.parts[0]...)
	// For the tracing part later own. We are going to start with the upper left most point.
	// First need to reverse b.parts[0]
	// Say we have for col1 (0,2),(0,3),(0,4),(0,5)
	// For col2 (1,2),(1,4),(1,5)
	points.Reverse(pts)
	// we should now have: (0,5),(0,4),(0,3),(0,2)
	//
	// we want (0,2), (1,2),(1,4),(1,5),(0,5),(0,4),(0,3)

	ring.Points = []maths.Pt{pts[len(pts)-1]}

	for i := range b.parts[1] {
		ring.Points = append(ring.Points, b.parts[1][i])
	}
	ring.Points = append(ring.Points, pts[:len(pts)-1]...)

	plen := len(ring.Points)
	if plen > 3 {
		switch {
		// Let's check the second to last pt, last pt, and the first pt to see if the last point can be dropped.
		case slopeCheck(ring.Points[plen-2], ring.Points[plen-1], ring.Points[0], x1, x2):
			ring.Points = ring.Points[:plen-1]
			// Let's check the  last pt, and the first two pts to see if the first point can be dropped.
		case slopeCheck(ring.Points[plen-1], ring.Points[0], ring.Points[1], x1, x2):
			ring.Points = ring.Points[1:]
		}
	}

	for i, pt := range ring.Points {
		if pt.X == x1 {
			y1s = append(y1s, YPart{
				Y:   pt.Y,
				Idx: i,
			})
			continue
		}
		if pt.X == x2 {
			y2s = append(y2s, YPart{
				Y:   pt.Y,
				Idx: i,
			})
			continue
		}

	}
	sort.Sort(YPartByY(y1s))
	sort.Sort(YPartByY(y2s))
	return ring, x1, y1s, x2, y2s
}

func (b *Builder) AddPts(l maths.Label, pts1, pts2 []maths.Pt) (ring Ring, x1 float64, y1s []YPart, x2 float64, y2s []YPart, new bool) {
	if b == nil {
		return ring, x1, y1s, x2, y2s, false
	}
	// Change in label means new ring to work on.
	if b.label == l {
		if len(pts1) > 1 && !b.parts[0][len(b.parts[0])-1].IsEqual(pts1[1]) {

			b.parts[0] = append(b.parts[0], pts1[1])
		}
		if len(pts2) > 1 && !b.parts[1][len(b.parts[1])-1].IsEqual(pts2[1]) {
			b.parts[1] = append(b.parts[1], pts2[1])
		}
		return ring, x1, y1s, x2, y2s, false
	}
	// If the current ring does not have any points in it, then just create a New ring
	new = len(b.parts[0]) != 0 && len(b.parts[1]) != 0
	if new {
		ring, x1, y1s, x2, y2s = b.CurrentRing()
	}
	b.label = l
	b.parts[0] = append([]maths.Pt{}, pts1...)
	b.parts[1] = append([]maths.Pt{}, pts2...)
	return ring, x1, y1s, x2, y2s, new
}
