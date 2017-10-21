package points

import (
	"github.com/terranodo/tegola/maths"
)

func RotatePos(pts []maths.Pt, pos int) {
	if pos == 0 {
		return
	}
	is := make([]maths.Pt, 0, len(pts))
	is = append(is, pts...)
	var j int
	for i := pos; i < len(pts); i++ {
		pts[j] = is[i]
		j++
	}
	for i := 0; i < pos; i++ {
		pts[j] = is[i]
		j++
	}
}

func RotateToLowestsFirst(pts []maths.Pt) {
	if len(pts) < 2 {
		return
	}
	bpts := ByXY(pts)
	//Find the lowests point.
	var fi int

	for i := range bpts[1:] {
		if !bpts.Less(fi, i+1) {
			fi = i + 1
		}
	}
	RotatePos(pts, fi)
}
