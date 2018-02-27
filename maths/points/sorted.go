package points

import (
	"sort"

	"github.com/go-spatial/tegola/maths"
)

func SortAndUnique(pts []maths.Pt) []maths.Pt {
	sort.Sort(ByXY(pts))
	lp := 0
	ptslen := len(pts)
	for i := 1; i < ptslen; i++ {
		if pts[i].IsEqual(pts[lp]) {
			continue
		}
		lp += 1
		if lp == i {
			continue
		}
		// found something that is not the same.
		copy(pts[lp:], pts[i:])
		// Adjust the length.
		ptslen -= (i - lp)
		i = lp
	}
	if ptslen > lp+1 {
		// Need to copy things over, and adjust the ptslen
		return pts[:lp+1]
	}
	return pts[:ptslen]
}
