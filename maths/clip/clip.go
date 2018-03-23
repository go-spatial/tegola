package clip

import (
	"sort"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/geom/cmp"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/lines"
)

type byxy [][2]float64

func (b byxy) Len() int      { return len(b) }
func (b byxy) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byxy) Less(i, j int) bool {
	if b[i][0] != b[j][0] {
		return b[i][0] < b[j][0]
	}
	return b[i][1] < b[j][1]
}

// IntersectPt returns the intersect point if one exists.
func intersectPt(clipbox *geom.Extent, ln [2][2]float64) (pts [][2]float64, ok bool) {
	lln := maths.NewLineWith2Float64(ln)
loop:
	for _, edge := range clipbox.Edges(nil) {
		eln := maths.NewLineWith2Float64(edge)
		if pt, ok := maths.Intersect(eln, lln); ok {
			// Only add if the point is actually on the line segment.
			if !eln.InBetween(pt) || !lln.InBetween(pt) {
				continue loop
			}

			// Only add if we have not see this point.
			for i := range pts {
				if pts[i][0] == pt.X && pts[i][1] == pt.Y {
					continue loop
				}
			}
			pts = append(pts, [2]float64{pt.X, pt.Y})
		}
	}
	sort.Sort(byxy(pts))
	return pts, len(pts) > 0
}

func LineString(linestr tegola.LineString, extent *geom.Extent) (ls []basic.Line, err error) {
	line := lines.FromTLineString(linestr)
	if len(line) == 0 {
		return ls, nil
	}

	var cpts [][2]float64
	lptIsIn := extent.ContainsPoint(line[0])
	if lptIsIn {
		cpts = append(cpts, line[0])
	}

	for i := 1; i < len(line); i++ {
		cptIsIn := extent.ContainsPoint(line[i])
		switch {
		case !lptIsIn && cptIsIn: // We are entering the extent region.
			if ipts, ok := intersectPt(extent, [2][2]float64{line[i-1], line[i]}); ok && len(ipts) > 0 {
				if len(ipts) == 1 {

					cpts = append(cpts, ipts[0])

				} else {
					isLess := cmp.PointLess(line[i-1], line[i])
					isCLess := cmp.PointLess(ipts[0], ipts[1])
					idx := 1
					if isLess == isCLess {
						idx = 0
					}
					cpts = append(cpts, ipts[idx])
				}

			}
			cpts = append(cpts, line[i])
		case !lptIsIn && !cptIsIn: // Both points are outside, but it's possible that they could be going straight through the regions.
			if ipts, ok := intersectPt(extent, [2][2]float64{line[i-1], line[i]}); ok && len(ipts) > 1 {
				// If this is the case return the line
				// We need to keep the direction.
				isLess := cmp.PointLess(line[i-1], line[i])
				isCLess := cmp.PointLess(ipts[0], ipts[1])
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
			if ipts, ok := intersectPt(extent, [2][2]float64{line[i-1], line[i]}); ok {
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
