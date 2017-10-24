package edgemap

import (
	"log"
	"runtime"
	"sort"
	"sync"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap/plyg"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

var numWorkers = 1

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
	numWorkers = runtime.NumCPU()
	log.Println("Number of workers:", numWorkers)
}

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

// destructure2 will split the ploygons up and split lines where they intersect. It will also, add a bounding box and a set of lines crossing from the end points of the bounding box to the center.
func destructure2(polygons [][]maths.Line, clipbox *points.BoundingBox) []maths.Line {
	// First we need to combine all the segments.
	segs := make(map[maths.Line]struct{})
	for i := range polygons {
		for _, ln := range polygons[i] {
			segs[ln.LeftRightMostAsLine()] = struct{}{}
		}
	}
	var segments []maths.Line
	if clipbox != nil {
		edges := clipbox.LREdges()
		segments = append(segments, edges[:]...)
	}
	for ln := range segs {
		segments = append(segments, ln)
	}
	if len(segments) <= 1 {
		return nil
	}
	return segments
}

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

type TriMP [][][]maths.Pt

func (t TriMP) TrianglesAsMP() [][][]maths.Pt { return t }

func GenerateTriangleGraph1(hm hitmap.Interface, adjustbb float64, polygons [][]maths.Line, extent float64) (TriMP, int) {
	clipbox := points.BoundingBox{-8, -8, extent + 8, extent + 8}
	segments := destructure2(polygons, &clipbox)
	if segments == nil {
		return nil, 0
	}
	cpolygons := destructure5(hm, &clipbox, segments)
	return TriMP(cpolygons), 0
}
