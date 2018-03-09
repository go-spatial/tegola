package makevalid

import (
	"context"
	"log"
	"sort"

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
func splitSegments(ctx context.Context, segments []maths.Line, clipbox *points.Extent) (lns [][2][2]float64, err error) {
	pts, err := splitPoints(ctx, segments)
	if err != nil {
		return nil, err
	}
	for _, pt := range pts {
		for i := 1; i < len(pt); i++ {
			ln := [2][2]float64{
				{pt[i-1].X, pt[i-1].Y},
				{pt[i].X, pt[i].Y},
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

// TODO: gdey — This is an intersect. We should move this onto points.Extent
// _adjustClipBox contracts the clipbox to just the region that the polygon exists in.
func _adjustClipBox(cpbx *points.Extent, plygs [][]maths.Line) (clipbox *points.Extent) {

	var pts [][2]float64
	for i := range plygs {
		for j := range plygs[i] {
			pts = append(
				pts,
				[2]float64{plygs[i][j][0].X, plygs[i][j][0].Y},
				[2]float64{plygs[i][j][1].X, plygs[i][j][1].Y},
			)

		}
	}
	if len(pts) == 0 {
		if cpbx == nil {
			return nil
		}
		return &points.Extent{cpbx[0], cpbx[1]}
	}

	// if there is a clipbox, let's adjust it to the polygon.
	// If what we are working on does not go outside one of the edges of the
	// clip box, let's bring that edge in. Basically, reduce the amount of
	// space we are dealing with.
	bb := points.Extent{pts[0], pts[0]}

	for i := 1; i < len(pts); i++ {
		// if the point is not in the clipbox we want to ignore it.
		// pt is outside of the x coords of clipbox.
		if clipbox != nil {
			if pts[i][0] < cpbx[0][0] || pts[i][0] > cpbx[1][0] {
				continue
			}
			// pt is outside of the y coords of clipbox.
			if pts[i][1] < cpbx[0][1] || pts[i][1] > cpbx[1][1] {
				continue
			}
		}
		if pts[i][0] < bb[0][0] {
			bb[0][0] = pts[i][0]
		}
		if pts[i][1] < bb[0][1] {
			bb[0][1] = pts[i][1]
		}
		if pts[i][0] > bb[1][0] {
			bb[1][0] = pts[i][0]
		}
		if pts[i][1] > bb[1][1] {
			bb[1][1] = pts[i][1]
		}
	}
	if cpbx == nil {
		return &bb
	}
	clipbox = &points.Extent{cpbx[0], cpbx[1]}
	if debug {
		log.Println("Before Clipbox:", clipbox)
	}
	if clipbox[0][0] < bb[0][0] {
		clipbox[0][0] = bb[0][0]
	}
	if clipbox[1][0] > bb[1][0] {
		clipbox[1][0] = bb[1][0]
	}
	if clipbox[0][1] < bb[0][1] {
		clipbox[0][1] = bb[0][1]
	}
	if clipbox[1][1] > bb[1][1] {
		clipbox[1][1] = bb[1][1]
	}
	return clipbox

}
