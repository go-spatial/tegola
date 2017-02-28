package region

import (
	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

/*
A region is made up of axises and a winding order. A region can hold other points along it's axises.

*/
type Region struct {
	list.List
	sentinelPoints [4]*list.Pt
	winding        maths.WindingOrder
	// The direction of the axis. true means it an axis that goes from smaller to bigger, otherwise it goes from bigger point to smaller point.
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
	r.winding = winding
	r.List.Init()
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

func (r *Region) Axis(idx int) *axis {
	s, e := idx%4, (idx+1)%4
	return &axis{
		region:      r,
		idx:         s,
		pt0:         r.sentinelPoints[s],
		pt1:         r.sentinelPoints[e],
		downOrRight: r.aDownOrRight[s],
		winding:     r.winding,
	}
}
func (r *Region) FirstAxis() *axis { return r.Axis(0) }
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
	return r.max.X >= pt.X && pt.X >= r.min.X &&
		r.max.Y >= pt.Y && pt.Y >= r.min.Y
}
func (r *Region) SentinalPoints() (pts []maths.Pt) {
	for _, p := range r.sentinelPoints {
		pts = append(pts, p.Point())
	}
	return pts
}
