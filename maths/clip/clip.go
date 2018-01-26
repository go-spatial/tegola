package clip

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/geom/cmp"
	"github.com/terranodo/tegola/maths/lines"
	"github.com/terranodo/tegola/maths/points"
)

func LineString(linestr tegola.LineString, extent *points.Extent) (ls []basic.Line, err error) {
	line := lines.FromTLineString(linestr)

	var cpts [][2]float64
	lptIsIn := extent.Contains(line[0])
	if lptIsIn {
		cpts = append(cpts, line[0])
	}

	for i := 1; i < len(line); i++ {
		cptIsIn := extent.Contains(line[i])
		switch {
		case !lptIsIn && cptIsIn: // We are entering the extent region.
			if ipts, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok && len(ipts) > 0 {
				if len(ipts) == 1 {

					cpts = append(cpts, ipts[0])

				} else {
					isLess := cmp.Point(line[i-1], line[i]) == cmp.Less
					isCLess := cmp.Point(ipts[0], ipts[1]) == cmp.Less
					idx := 1
					if isLess == isCLess {
						idx = 0
					}
					cpts = append(cpts, ipts[idx])
				}

			}
			cpts = append(cpts, line[i])
		case !lptIsIn && !cptIsIn: // Both points are outside, but it's possible that they could be going straight through the regions.
			if ipts, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok && len(ipts) > 1 {
				// If this is the case return the line
				// We need to keep the direction.
				isLess := cmp.Point(line[i-1], line[i]) == cmp.Less
				isCLess := cmp.Point(ipts[0], ipts[1]) == cmp.Less
				f, s := 0, 1
				if isLess != isCLess {
					f, s = 1, 0
				}
				ls = append(ls, basic.NewLineFrom2Float64(ipts[f], ipts[s]))

			}
			cpts = cpts[:0]
		case lptIsIn && cptIsIn: // Both points are in, just add the new point.
			cpts = append(cpts, line[i])
		case lptIsIn && !cptIsIn: // We are headed out of the region.
			if ipts, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok {
				_ = ipts
				// It's is possible that our intersect point is the same as our lpt.
				// if this is the case we need to ignore it.
				lpt := cpts[len(cpts)-1]
				for _, ipt := range ipts {
					if ipt[0] != lpt[0] || ipt[1] != lpt[1] {
						cpts = append(cpts, ipt)
					}
				}
			}
			// Time to add this line to our set of lines, and reset
			// the new line.
			ls = append(ls, basic.NewLineFrom2Float64(cpts...))
			cpts = cpts[:0]
		}
		lptIsIn = cptIsIn
	}
	if len(cpts) > 0 {
		ls = append(ls, basic.NewLineFrom2Float64(cpts...))
	}
	return ls, nil
}
