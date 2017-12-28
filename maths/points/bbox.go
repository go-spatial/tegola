package points

import (
	"fmt"
	"math"

	"github.com/terranodo/tegola/maths"
)

type BoundingBox [4]float64

func (bb BoundingBox) PointAt(i int) maths.Pt {
	if i >= 4 {
		i = i % 4
	}
	switch i {
	case 0:
		return maths.Pt{bb[0], bb[1]}
	case 1:
		return maths.Pt{bb[0], bb[3]}
	case 2:
		return maths.Pt{bb[2], bb[3]}
	default:
		return maths.Pt{bb[2], bb[1]}
	}
}

func (bb BoundingBox) ContainBB(bb1 [4]float64) bool {
	return bb[0] <= bb1[0] && // bb1 minx is bigger or the same as bb
		bb[1] <= bb1[1] && // bb1 miny is bigger or the same as bb
		bb[2] >= bb1[2] && // bb1 maxx is smaller or the same as bb
		bb[3] >= bb1[3] // bb1 maxy is smaller or the same as bb

}

func (bb BoundingBox) LREdges() [4]maths.Line {
	return [4]maths.Line{
		{maths.Pt{bb[0], bb[1]}, maths.Pt{bb[2], bb[1]}}, // MinX,MinY -> MaxX,MinY
		{maths.Pt{bb[0], bb[1]}, maths.Pt{bb[0], bb[3]}}, // MinX,MinY -> MinX,MaxY
		{maths.Pt{bb[0], bb[3]}, maths.Pt{bb[2], bb[3]}}, // MinX,MaxY -> MaxX,MaxY
		{maths.Pt{bb[2], bb[1]}, maths.Pt{bb[2], bb[3]}}, // MaxX,MinY -> MaxX,MaxX
	}
}

func (bb BoundingBox) Contains(pt maths.Pt) bool {
	return bb[0] <= pt.X && pt.X <= bb[2] &&
		bb[1] <= pt.Y && pt.Y <= bb[3]
}
func (bb BoundingBox) ContainsLine(l maths.Line) bool {
	return bb.Contains(l[0]) && bb.Contains(l[1])
}

func (bb BoundingBox) Area() float64 {
	return math.Abs((bb[2] - bb[0]) * (bb[3] - bb[1]))
}

func BBoxFloat64(pts ...[2]float64) (bb [2][2]float64, err error) {
	if len(pts) == 0 {
		return bb, fmt.Errorf("No points given.")
	}
	bb = [2][2]float64{
		{pts[0][0], pts[0][1]},
		{pts[0][0], pts[0][1]},
	}
	for i := 1; i < len(pts); i++ {
		if pts[i][0] < bb[0][0] {
			bb[0][0] = pts[i][0]
		}
		if pts[i][1] < bb[0][1] {
			bb[0][1] = pts[i][1]
		}
		if pts[i][0] > bb[1][0] {
			bb[1][0] = pts[i][0]
		}
		if pts[i][1] > bb[1][1] {
			bb[1][1] = pts[i][1]
		}
	}
	return bb, nil
}

// TODO:gdey — should we return an error?
func BBox(pts []maths.Pt) (bb [4]float64) {
	if len(pts) == 0 {
		return bb
	}
	bb = [4]float64{pts[0].X, pts[0].Y, pts[0].X, pts[0].Y}
	for _, pt := range pts[1:] {
		if pt.X < bb[0] {
			bb[0] = pt.X
		}
		if pt.Y < bb[1] {
			bb[1] = pt.Y
		}
		if pt.X > bb[2] {
			bb[2] = pt.X
		}
		if pt.Y > bb[3] {
			bb[3] = pt.Y
		}
	}
	return bb
}
