package edgemap

import (
	"log"
	"runtime"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

var numWorkers = 1

func init() {
	numWorkers = runtime.NumCPU()
	log.Println("Number of workers:", numWorkers)
}

/*
type presortedByXYLine []maths.Line

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
*/

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
