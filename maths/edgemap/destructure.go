package edgemap

import (
	"sort"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

func InsureConnected(polygons ...[]maths.Line) (ret [][]maths.Line) {
	return insureConnected(polygons...)
}

// insureConnected will add a connecting line as needed to the given polygons. If there is only one line in a polygon, it will be left alone.
func insureConnected(polygons ...[]maths.Line) (ret [][]maths.Line) {
	ret = make([][]maths.Line, len(polygons))
	for i := range polygons {
		ln := len(polygons[i])
		if ln == 0 {
			continue
		}
		ret[i] = append(ret[i], polygons[i]...)
		if ln == 1 {
			continue
		}
		if !polygons[i][ln-1][1].IsEqual(polygons[i][0][0]) {
			ret[i] = append(ret[i], maths.Line{polygons[i][ln-1][1], polygons[i][0][0]})
		}
	}
	return ret
}

func Destructure(polygons [][]maths.Line) (lines []maths.Line) {
	return destructure(polygons)
}

// desctucture will split the given polygons into their composit lines, breaking up lines at intersection points. It will remove lines that overlap as well. Polygons need to be fully connected before calling this function.
func destructure(polygons [][]maths.Line) (lines []maths.Line) {

	// First we need to combine all the segments.
	var segments []maths.Line
	{
		segs := make(map[maths.Line]struct{})
		for i := range polygons {
			for _, ln := range polygons[i] {
				segs[ln.LeftRightMostAsLine()] = struct{}{}
			}
		}
		for ln := range segs {
			segments = append(segments, ln)
		}
		if len(segments) <= 1 {
			return segments
		}
		sort.Sort(maths.ByXYLine(segments))
	}

	// linesToSplit holds a list of points for that segment to be split at. This list will have to be
	// ordered and deuped.
	splitPts := make([][]maths.Pt, len(segments))

	maths.FindIntersects(segments, func(src, dest int, ptfn func() maths.Pt) bool {

		sline, dline := segments[src], segments[dest]

		pt := ptfn() // left most point.
		pt.X = float64(int64(pt.X))
		pt.Y = float64(int64(pt.Y))
		if !(pt.IsEqual(sline[0]) || pt.IsEqual(sline[1])) {
			splitPts[src] = append(splitPts[src], pt)
		}
		if !(pt.IsEqual(dline[0]) || pt.IsEqual(dline[1])) {
			splitPts[dest] = append(splitPts[dest], pt)
		}
		return true
	})

	for i := range segments {
		if splitPts[i] == nil {
			lines = append(lines, segments[i].LeftRightMostAsLine())
			continue
		}
		sort.Sort(points.ByXY(splitPts[i]))
		lidx, ridx := maths.Line(segments[i]).XYOrderedPtsIdx()
		lpt, rpt := segments[i][lidx], segments[i][ridx]
		for j := range splitPts[i] {
			if lpt.IsEqual(splitPts[i][j]) {
				// Skipp dups.
				continue
			}
			lines = append(lines, maths.Line{lpt, splitPts[i][j]}.LeftRightMostAsLine())
			lpt = splitPts[i][j]
		}
		if !lpt.IsEqual(rpt) {
			lines = append(lines, maths.Line{lpt, rpt}.LeftRightMostAsLine())
		}
	}

	sort.Sort(maths.ByXYLine(lines))
	return lines
}
