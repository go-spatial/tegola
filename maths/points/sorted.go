package points

import (
	"sort"

	"github.com/terranodo/tegola/maths"
)

func SortAndUnique(pts []maths.Pt) (spts []maths.Pt) {
	sort.Sort(ByXY(pts))
	lpt := pts[0]
	spts = append(spts, lpt)
	for _, pt := range pts[1:] {
		if lpt.IsEqual(pt) {
			continue
		}
		spts = append(spts, pt)
		lpt = pt
	}
	return spts
}
