package makevalid

import (
	"sort"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

type colPtMap struct {
	X2Pt     map[float64][]maths.Pt
	Pt2MaxY  map[maths.Pt]int64
	X        []float64
	maxy     float64
	cMaxY100 int64
}

func orderPoints(p1, p2 maths.Pt) (maths.Pt, maths.Pt) {
	if p1.X < p2.X {
		return p1, p2
	}
	if p1.X > p2.X {
		return p2, p1
	}
	if p1.Y <= p2.Y {
		return p1, p2
	}
	return p2, p1

}

func (cm *colPtMap) add2Map(p1, p2 maths.Pt) {
	cm.X2Pt[p1.X] = append(cm.X2Pt[p1.X], p1)
	cm.X2Pt[p2.X] = append(cm.X2Pt[p2.X], p2)
	if p1.X == p2.X {
		// Nothing more to do.
		return
	}
	// Make sure the p1 is to the left of p2
	p1, p2 = orderPoints(p1, p2)
	oldy100, ok := cm.Pt2MaxY[p1]
	y100 := cm.maxY100Val(p2.Y)
	if !ok || oldy100 < y100 {
		cm.Pt2MaxY[p1] = y100
	}
}

func (cm *colPtMap) maxY100Val(y float64) int64 {
	y100 := int64(y * 100)
	if y100 < cm.cMaxY100 {
		return y100
	}
	return cm.cMaxY100
}

func _NewColPtMap(pts [][]maths.Pt, maxy float64) *colPtMap {
	var cm colPtMap
	cm.X2Pt = make(map[float64][]maths.Pt)
	cm.Pt2MaxY = make(map[maths.Pt]int64)
	cm.maxy = maxy
	cm.cMaxY100 = int64(maxy * 100)
	for i := range pts {
		for j := 1; j < len(pts[i]); j++ {
			cm.add2Map(pts[i][j-1], pts[i][j])
		}
	}
	for i := range cm.X2Pt {
		cm.X = append(cm.X, i)
	}
	sort.Float64s(cm.X)
	for _, x := range cm.X {
		cm.X2Pt[x] = points.SortAndUnique(cm.X2Pt[x])
	}

	return &cm
}
