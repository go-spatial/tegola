// +build xsweep

package edgemap

import (
	"sort"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

type presortedByXY []Line

func (t presortedByXYLine) Len() int      { return len(t) }
func (t presortedByXYLine) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t presortedByXYLine) Less(i, j int) bool {
	switch {

	// Test the x-coord first
	case t[i][0].X > t[j][0].X:
		return false
	case t[i][0].X < t[j][0].X:
		return true

		// Test the y-coord second
	case t[i][0].Y > t[j][0].Y:
		return false
	case t[i][0].Y < t[j][0].Y:
		return true

	// Test the x-coord first
	case t[i][1].X > t[j][1].X:
		return false
	case t[i][1].X < t[j][1].X:
		return true

		// Test the y-coord second
	case t[i][1].Y > t[j][1].Y:
		return false
	case t[i][1].Y < t[j][1].Y:
		return true

	}
	return false
}

// destructure2 will split the ploygons up and split lines where they intersect. It will also, add a bounding box and a set of lines crossing from the end points of the bounding box to the center.
func destructure2(adjustbb float64, polygons [][]maths.Line) (lines []maths.Line, unconstrained []int) {
	var xs = make(map[float64]struct{})
	var minpt, maxpt maths.Pt
	// First we need to combine all the segments.
	var segments []maths.Line
	{
		var lrln maths.Line
		var minset bool
		var minx, miny, maxx, maxy float64
		segs := make(map[maths.Line]struct{})
		for i := range polygons {
			for _, ln := range polygons[i] {
				lrln = ln.LeftRightMostAsLine
				segs[ln.LeftRightMostAsLine()] = struct{}{}
				xs[lrln[0].X] = struct{}{}
				xs[lrln[1].X] = struct{}{}
				// get the min and max points
				if !minset {
					minx = lrln[0].X
					miny = lrln[0].Y
					maxx = lrln[0].X
					maxy = lrln[0].Y
				}
				if minx > lrln[0].X {
					minx = lrln[0].X
				}
				if miny > lrln[0].Y {
					miny = lrln[0].Y
				}
				if minx > lrln[1].X {
					minx = lrln[1].X
				}
				if miny > lrln[1].Y {
					miny = lrln[1].Y
				}

				if maxx < lrln[0].X {
					maxx = lrln[0].X
				}
				if maxy < lrln[0].Y {
					maxy = lrln[0].Y
				}
				if maxx < lrln[1].X {
					maxx = lrln[1].X
				}
				if maxy < lrln[1].Y {
					maxy = lrln[1].Y
				}

			}
		}
		minpt, maxpt = maths.Pt{minx, miny}, maths.Pt{maxx, maxy}
		for ln := range segs {
			segments = append(segments, ln)
		}
		if len(segments) <= 1 {
			return segments
		}
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

func NewEMXSweep(polygons [][]maths.Line) (*EM, error) {

}

func (em *EM) Triangulate() {
	//defer log.Println("Done with Triangulate")
	keys := em.Keys
	lnkeys := len(keys) - 1
	var lines []maths.Line

	//log.Println("Starting to Triangulate. Keys", len(keys))
	// We want to run through all the keys up to the last key, to generating possible edges, and then
	// collecting the ones that don't intersect with the edges in the map already.
	for i := 0; i < lnkeys; i++ {
		lookup := em.Map[keys[i]]
		var possibleEdges []maths.Line
		for j := i + 1; j < len(keys); j++ {
			if _, ok := lookup[keys[j]]; ok {
				// Already have an edge with this point
				continue
			}
			l := maths.Line{keys[i], keys[j]}
			possibleEdges = append(possibleEdges, l)
		}

		// Now we need to do a line sweep to see which of the possible edges we want to keep.
		lines = append([]maths.Line{}, possibleEdges...)
		offset := len(lines)
		lines = append(lines, em.Segments...)
		skiplines := make([]bool, offset)
		findIntersects(lines, skiplines)

		// Add the remaining possible Edges to the edgeMap.
		lines = lines[:0]
		for i := range possibleEdges {
			if skiplines[i] {
				continue
			}
			lines = append(lines, possibleEdges[i])
		}
		em.AddLine(false, true, false, lines...)
	}
}
