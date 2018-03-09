package makevalid

import (
	"context"
	"sort"

	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/points"
)

func allCoordForPts(idx int, pts ...[2]float64) []float64 {
	if idx != 0 && idx != 1 {
		panic("idx can only be 0 or 1 for x, and y")
	}

	fs := make([]float64, 0, len(pts))
	for i := range pts {
		fs = append(fs, pts[i][idx])
	}
	return fs
}

func sortUniqueF64(fs []float64) []float64 {
	if len(fs) == 0 {
		return fs
	}

	sort.Float64s(fs)
	count := 0
	for i := range fs {
		if fs[count] == fs[i] {
			continue
		}

		count++
		fs[count] = fs[i]
	}

	return fs[:count+1]
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
		pts[src] = append(pts[src], pt)
		pts[dest] = append(pts[dest], pt)
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
func splitSegments(ctx context.Context, segments []maths.Line, clipbox *geom.Extent) (lns [][2][2]float64, err error) {
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
func splitLines(ctx context.Context, segments []maths.Line, clipbox *geom.Extent) ([]maths.Line, error) {
	lns, err := splitSegments(ctx, segments, clipbox)
	if err != nil {
		return nil, err
	}
	return maths.NewLinesFloat64(lns...), nil

}
