package region

import (
	"github.com/go-spatial/tegola/container/singlelist/point/list"
	"github.com/go-spatial/tegola/maths"
)

/*
A region is made up of axises and a winding order. A region can hold other points along it's axises.

*/
type Region struct {
	list.List
	sentinelPoints [4]*list.Pt
	winding        maths.WindingOrder
	// The direction of the Axis. true means it an Axis that goes from smaller to bigger, otherwise it goes from bigger point to smaller point.
	aDownOrRight [4]bool
	max, min     maths.Pt
}

/*
  Winding order:

  Clockwise

		        1
		1pt   _____  2pt
		     |     |
		   0 |     | 2
		     |_____|
		0pt     3    3pt

  Counter Clockwise

		        3
		0pt   _____  3pt
		     |     |
		   0 |     | 2
		     |_____|
		1pt     1    2pt
*/

// New creates a new region, initilization paramters as needed.
func New(winding maths.WindingOrder, Min, Max maths.Pt) *Region {
	return new(Region).Init(winding, Min, Max)
}

// Init initilizes the region struct.
func (r *Region) Init(winding maths.WindingOrder, Min, Max maths.Pt) *Region {
	//log.Println("Creating new clipping region ", Min, Max)
	r.winding = winding
	//r.List.Init()
	r.max = Max
	r.min = Min
	var pts [4][2]float64
	if winding == maths.Clockwise {
		/*
			  Clockwise

			   MinX,MinY    1   MaxX,MinY
					1pt   _____  2pt
					     |     |
					   0 |     | 2
					     |_____|
					0pt     3    3pt
			   MinX,MaxY       MaxX,MaxY
		*/
		pts = [4][2]float64{
			[2]float64{Min.X, Max.Y},
			[2]float64{Min.X, Min.Y},
			[2]float64{Max.X, Min.Y},
			[2]float64{Max.X, Max.Y},
		}
		r.aDownOrRight = [4]bool{false, true, true, false}
	} else {
		/*
			  Counter Clockwise

			   MinX,MinY    3   MaxX,MinY
					0pt   _____  3pt
					     |     |
					   0 |     | 2
					     |_____|
					1pt     1    2pt
			   MinX,MaxY       MaxX,MaxY
		*/
		pts = [4][2]float64{[2]float64{Min.X, Min.Y}, [2]float64{Min.X, Max.Y}, [2]float64{Max.X, Max.Y}, [2]float64{Max.X, Min.Y}}
		r.aDownOrRight = [4]bool{true, true, false, false}
	}
	for i, pt := range pts {
		point := list.NewPoint(pt[0], pt[1])
		r.sentinelPoints[i] = point
		r.PushBack(point)
	}
	return r
}

func (r *Region) Axis(idx int) *Axis {
	s, e := idx%4, (idx+1)%4
	return &Axis{
		region:      r,
		idx:         s,
		pt0:         r.sentinelPoints[s],
		pt1:         r.sentinelPoints[e],
		downOrRight: r.aDownOrRight[s],
		winding:     r.winding,
	}
}
func (r *Region) FirstAxis() *Axis { return r.Axis(0) }
func (r *Region) LineString() []float64 {
	return []float64{
		r.sentinelPoints[0].Pt.X, r.sentinelPoints[0].Pt.Y,
		r.sentinelPoints[1].Pt.X, r.sentinelPoints[1].Pt.Y,
		r.sentinelPoints[2].Pt.X, r.sentinelPoints[2].Pt.Y,
		r.sentinelPoints[3].Pt.X, r.sentinelPoints[3].Pt.Y,
	}
}

func (r *Region) Max() maths.Pt                    { return r.max }
func (r *Region) Min() maths.Pt                    { return r.min }
func (r *Region) WindingOrder() maths.WindingOrder { return r.winding }
func (r *Region) Contains(pt maths.Pt) bool {
	return r.max.X > pt.X && pt.X > r.min.X &&
		r.max.Y > pt.Y && pt.Y > r.min.Y
}
func (r *Region) SentinalPoints() (pts []maths.Pt) {
	for _, p := range r.sentinelPoints {
		pts = append(pts, p.Point())
	}
	return pts
}

// Intersect holds the intersect point and the direction of the vector it's on. Into or out of the clipping region.
type Intersect struct {
	// Pt is the intersect point.
	Pt maths.Pt
	// Is the vector this point is on heading into the region.
	Inward bool
	// Index of the Axis this point was found on.
	Idx       int
	isNotZero bool
}

// Intersections returns zero to four intersections points.
// You should remove any duplicate and cancelling intersections points afterwards.
func (r *Region) Intersections(l maths.Line) (out []Intersect, Pt1Placement, Pt2Placement PlacementCode) {
	pt1, pt2 := l[0], l[1]

	if r.Contains(pt1) && r.Contains(pt2) {
		return out, Pt1Placement, Pt2Placement
	}
	var ai [4]Intersect
	for i := 0; i < len(ai); i++ {

		a := r.Axis(i)

		Pt1Placement |= a.Placement(pt1)
		Pt2Placement |= a.Placement(pt2)

		pt, doesIntersect := a.Intersect(l)
		if !doesIntersect {
			continue
		}
		inward, err := a.IsInward(l)
		if err != nil {
			continue
		}

		out = append(out, Intersect{
			Pt:        pt,
			Inward:    inward,
			Idx:       i,
			isNotZero: true,
		})

	}
	return out, Pt1Placement, Pt2Placement
}
