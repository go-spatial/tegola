package clip

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths/lines"
	"github.com/terranodo/tegola/maths/points"
)

func linestring2floats(l tegola.LineString) (ls []float64) {
	for _, p := range l.Subpoints() {
		ls = append(ls, p.X(), p.Y())
	}
	return ls
}

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
			if ipt, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok && len(ipt) > 0 {
				cpts = append(cpts, ipt[0])
			}
			cpts = append(cpts, line[i])
		case !lptIsIn && !cptIsIn: // Both points are outside, but it's possible that they could be going straight through the regions.
			if ipt, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok && len(ipt) > 1 {
				ls = append(ls, basic.NewLineFrom2Float64(ipt...))
			}
			cpts = cpts[:0]
		case lptIsIn && cptIsIn: // Both points are in, just add the new point.
			cpts = append(cpts, line[i])
		case lptIsIn && !cptIsIn: // We are headed out of the region.
			if ipt, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok {
				_ = ipt
				cpts = append(cpts, ipt...)
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
