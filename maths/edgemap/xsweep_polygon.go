package edgemap

import (
	"sort"
	"sync"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap/plyg"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

func destructure5(hm hitmap.Interface, clipbox *points.BoundingBox, segments []maths.Line) [][][]maths.Pt {

	var lines []maths.Line

	// linesToSplit holds a list of points for that segment to be split at. This list will have to be
	// ordered and deuped.
	splitPts := make([][]maths.Pt, len(segments))

	maths.FindIntersects(segments, func(src, dest int, ptfn func() maths.Pt) bool {

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
		if !(pt.IsEqual(sline[0]) || pt.IsEqual(sline[1])) {
			splitPts[src] = append(splitPts[src], pt)
		}
		if !(pt.IsEqual(dline[0]) || pt.IsEqual(dline[1])) {
			splitPts[dest] = append(splitPts[dest], pt)
		}
		return true
	})

	var xs []float64
	var uxs []float64
	miny, maxy := segments[0][1].Y, segments[0][1].Y
	{
		mappts := make(map[maths.Pt]struct{}, len(segments)*2)
		var lrln maths.Line

		for i := range segments {
			if splitPts[i] == nil {
				lrln = segments[i].LeftRightMostAsLine()
				if !clipbox.ContainsLine(lrln) {
					// Outside of the clipping area.
					continue
				}
				mappts[lrln[0]] = struct{}{}
				mappts[lrln[1]] = struct{}{}
				xs = append(xs, lrln[0].X, lrln[1].X)

				if lrln[0].Y < miny {
					miny = lrln[0].Y
				}
				if lrln[0].Y > maxy {
					maxy = lrln[0].Y
				}
				if lrln[1].Y < miny {
					miny = lrln[0].Y
				}
				if lrln[1].Y > maxy {
					maxy = lrln[1].Y
				}
				lines = append(lines, lrln)
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
				lrln = maths.Line{lpt, splitPts[i][j]}.LeftRightMostAsLine()
				lpt = splitPts[i][j]
				if !clipbox.ContainsLine(lrln) {
					// Outside of the clipping area.
					continue
				}
				mappts[lrln[0]] = struct{}{}
				mappts[lrln[1]] = struct{}{}
				xs = append(xs, lrln[0].X, lrln[1].X)
				lines = append(lines, lrln)
			}
			if !lpt.IsEqual(rpt) {
				lrln = maths.Line{lpt, rpt}.LeftRightMostAsLine()
				if !clipbox.ContainsLine(lrln) {
					// Outside of the clipping area.
					continue
				}
				mappts[lrln[0]] = struct{}{}
				mappts[lrln[1]] = struct{}{}
				xs = append(xs, lrln[0].X, lrln[1].X)
				lines = append(lines, lrln)
			}
		}
	}

	sort.Float64s(xs)
	minx, maxx := xs[0], xs[len(xs)-1]
	xs = append(append([]float64{minx}, xs...), maxx)
	lx := xs[0]
	uxs = append(uxs, lx)
	lines = append(lines, maths.Line{maths.Pt{lx, miny}, maths.Pt{lx, maxy}})
	// Draw lines along each x to make columns
	for _, x := range xs[1:] {
		if x == lx {
			continue
		}
		lines = append(lines,
			maths.Line{maths.Pt{x, miny}, maths.Pt{x, maxy}},
		)
		lx = x
		uxs = append(uxs, x)
	}

	splitPts = make([][]maths.Pt, len(lines))

	maths.FindIntersects(lines, func(src, dest int, ptfn func() maths.Pt) bool {

		sline, dline := lines[src], lines[dest]
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
		if !(pt.IsEqual(sline[0]) || pt.IsEqual(sline[1])) {
			splitPts[src] = append(splitPts[src], pt)
		}
		if !(pt.IsEqual(dline[0]) || pt.IsEqual(dline[1])) {
			splitPts[dest] = append(splitPts[dest], pt)
		}
		return true
	})

	var x2pts = make(map[float64][]maths.Pt)
	var pt2MaxY = make(map[maths.Pt]int64)
	var add2Maps = func(pt1, pt2 maths.Pt) {
		x2pts[pt1.X] = append(x2pts[pt1.X], pt1)
		x2pts[pt2.X] = append(x2pts[pt2.X], pt2)
		if pt2.X != pt1.X {
			if y1, ok := pt2MaxY[pt1]; !ok || y1 < int64(pt2.Y*100) {
				pt2MaxY[pt1] = int64(pt2.Y * 100)
			}
		}
	}
	{
		for i := range lines {
			if splitPts[i] == nil {
				// We are not splitting the line.
				add2Maps(lines[i][0], lines[i][1])
				continue
			}

			sort.Sort(points.ByXY(splitPts[i]))
			lidx, ridx := lines[i].XYOrderedPtsIdx()
			lpt, rpt := lines[i][lidx], lines[i][ridx]
			for j := range splitPts[i] {
				if lpt.IsEqual(splitPts[i][j]) {
					// Skipp dups.
					continue
				}
				add2Maps(lpt, splitPts[i][j])
				lpt = splitPts[i][j]
			}
			if !lpt.IsEqual(rpt) {
				add2Maps(lpt, rpt)
			}
		}
	}

	for i := range uxs {
		x2pts[uxs[i]] = points.SortAndUnique(x2pts[uxs[i]])
	}

	var wg sync.WaitGroup
	var idChan = make(chan int)
	var lenuxs = len(uxs) - 1

	var ringCols = make([]plyg.RingCol, lenuxs)

	var worker = func(id int) {
		for i := range idChan {
			ringCols[i] = plyg.BuildRingCol(
				hm,
				x2pts[uxs[i]],
				x2pts[uxs[i+1]],
				pt2MaxY,
			)
		}
		wg.Done()
	}
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go worker(i)
	}
	for i := 0; i < lenuxs; i++ {
		idChan <- i
	}

	close(idChan)
	wg.Wait()

	plygs := plyg.GenerateMultiPolygon(ringCols)
	return plygs
}
