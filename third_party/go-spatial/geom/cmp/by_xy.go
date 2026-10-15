package cmp

import "github.com/go-spatial/geom"

type ByXY [][2]float64

func (xy ByXY) Less(i, j int) bool { return XYLessPoint(xy[i], xy[j]) }
func (xy ByXY) Swap(i, j int)      { xy[i], xy[j] = xy[j], xy[i] }
func (xy ByXY) Len() int           { return len(xy) }

type PointByXY []geom.Point

func (xy PointByXY) Less(i, j int) bool {
	return XYLessPoint([2]float64(xy[i]), [2]float64(xy[j]))
}
func (xy PointByXY) Swap(i, j int) { xy[i], xy[j] = xy[j], xy[i] }
func (xy PointByXY) Len() int      { return len(xy) }

// bySizeXY is for sorting polygons. There are a few things we need to take
// in to consideration when sorting polygons.
// 1. The size of the ring.
// 2. If the size is the same, then we need to RotateToLeftMostPoint, and then compare the 1st point in the line string.
type bySubRingSizeXY [][][2]float64

func (xy bySubRingSizeXY) Less(i, j int) bool {
	// The first ring is special. It should always be the first ring.
	switch {
	case i == 0:
		return true
	case j == 0:
		return false
	case len(xy[i]) != len(xy[j]):
		return len(xy[i]) < len(xy[j])
	default:
		// if they are the same length we need to use the min point to determine which goes where.
		mi, mj := FindMinPointIdx(xy[i]), FindMinPointIdx(xy[j])
		return XYLessPoint(xy[i][mi], xy[j][mj])
	}
}

func (xy bySubRingSizeXY) Len() int      { return len(xy) }
func (xy bySubRingSizeXY) Swap(i, j int) { xy[i], xy[j] = xy[j], xy[i] }

type byPolygonMainSizeXY [][][][2]float64

func (xy byPolygonMainSizeXY) Less(i, j int) bool {
	if len(xy[i]) == 0 {
		return true
	}
	if len(xy[j]) == 0 {
		return false
	}
	if len(xy[i][0]) != len(xy[j][0]) {
		return len(xy[i][0]) < len(xy[j][0])
	}
	mi, mj := FindMinPointIdx(xy[i][0]), FindMinPointIdx(xy[j][0])
	return XYLessPoint(xy[i][0][mi], xy[j][0][mj])
}
func (xy byPolygonMainSizeXY) Len() int      { return len(xy) }
func (xy byPolygonMainSizeXY) Swap(i, j int) { xy[i], xy[j] = xy[j], xy[i] }
