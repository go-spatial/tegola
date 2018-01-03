package makevalid

import (
	"context"
	"sort"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

// destructure will split a polygon up into line segments, that do not interset. If a polygon contains line segments that do interset those line segments will be split at the intersect point. If a clipbox is given, additional line segments representing the boundries of the clip will be added. Any line segments outside of the clip box will be disgarded.
/*
func destructure (polygons ...[][][2]float64)(ret [][2][2]float64){
}
*/

/*
func appendUniquePt(pts []maths.Pt, pt maths.Pt) []maths.Pt {
	for _, p := range pts {
		if p.IsEqual(pt) {
			// point is already in the array.
			return pts
		}
	}
	return append(pts, pt)
}
*/

func allCoordForPts(idx int, pts ...[2]float64) (fs []float64) {
	if idx != 0 && idx != 1 {
		panic("idx can only be 0 or 1 for x, and y")
	}
	for i := range pts {
		fs = append(fs, pts[i][idx])
	}
	return fs
}

func sortUniqueF64(fs []float64) []float64 {
	sort.Float64s(fs)
	lf := 0
	fslen := len(fs)
	for i := 1; i < fslen; i++ {
		if fs[i] == fs[lf] {
			continue
		}
		lf += 1
		if lf == i {
			continue
		}
		// found something that is not the same.
		copy(fs[lf:], fs[i:])
		fslen -= (i - lf)
		i = lf
	}
	if fslen > lf+1 {
		// Need to copy things over, and adjust the fslen
		return fs[:lf+1]
	}
	return fs[:fslen]
}

// splitPoints will find the points amount the lines that lines should be split at.
func splitPoints(ctx context.Context, segments []maths.Line) (pts [][]maths.Pt, err error) {
	// For each segment we keep a list of point that that segment needs to split
	// at.
	pts = make([][]maths.Pt, len(segments))
	for i := range segments {
		pts[i] = append(pts[i], segments[i][0], segments[i][1])
	}

	maths.FindIntersects(segments, func(src, dest int, ptfn func() maths.Pt) bool {

		if ctx.Err() != nil {
			// Want to exit early from the loop, the ctx got cancelled.
			return false
		}

		sline, dline := segments[src], segments[dest]

		// Check to see if the end points of sline and dline intersect?
		if (sline[0].IsEqual(dline[0])) ||
			(sline[0].IsEqual(dline[1])) ||
			(sline[1].IsEqual(dline[0])) ||
			(sline[1].IsEqual(dline[1])) {
			return true
		}

		pt := ptfn().Round() // left most point.
		if !sline.InBetween(pt) || !dline.InBetween(pt) {
			return true
		}
		/*
			log.Println("Checking src end points:", !pt.IsEqual(sline[0]), !pt.IsEqual(sline[1]))
			// Check the end points.
			if !pt.IsEqual(sline[0]) && !pt.IsEqual(sline[1]) {
		*/
		pts[src] = append(pts[src], pt)
		/*
			}
			log.Println("Checking dest end points:", !pt.IsEqual(dline[0]), !pt.IsEqual(dline[1]))
			if !pt.IsEqual(dline[0]) && !pt.IsEqual(dline[1]) {
		*/
		pts[dest] = append(pts[dest], pt)
		/*
			}
		*/
		return true
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range pts {
		// Sort the items.
		pts[i] = points.SortAndUnique(pts[i])
	}
	return pts, nil

}
func splitSegments(ctx context.Context, segments []maths.Line, clipbox *points.Extent) (lns [][2][2]float64, err error) {
	pts, err := splitPoints(ctx, segments)
	if err != nil {
		return nil, err
	}
	for _, pt := range pts {
		for i := 1; i < len(pt); i++ {
			ln := [2][2]float64{
				[2]float64{pt[i-1].X, pt[i-1].Y},
				[2]float64{pt[i].X, pt[i].Y},
			}
			// if clipbox is provided use it to filter out the  lines.
			if clipbox != nil && !clipbox.ContainsLine(ln) {
				continue
			}
			lns = append(lns, ln)
		}
	}
	return lns, err
}

func allPointsForSegments(segments [][2][2]float64) (pts [][2]float64) {
	for i := range segments {
		pts = append(pts, segments[i][0], segments[i][1])
	}
	return pts
}

// splitLines will split the given line segments at intersection points if they intersect at any point other then the end.
func splitLines(ctx context.Context, segments []maths.Line, clipbox *points.Extent) ([]maths.Line, error) {
	lns, err := splitSegments(ctx, segments, clipbox)
	if err != nil {
		return nil, err
	}
	return maths.NewLinesFloat64(lns...), nil

}
