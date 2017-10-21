// +build xsweep

package edgemap

import (
	"log"
	"runtime"

	"sort"
	"sync"

	"github.com/gdey/gotrace"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

var numWorkers = 1

func init() {
	numWorkers = runtime.NumCPU()
	log.Println("Number of workers:", numWorkers)
}

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

// destructure2 will split the ploygons up and split lines where they intersect. It will also, add a bounding box and a set of lines crossing from the end points of the bounding box to the center.
func destructure2(polygons [][]maths.Line, extent float64) []maths.Line {
	// First we need to combine all the segments.
	segs := make(map[maths.Line]struct{})
	for i := range polygons {
		for _, ln := range polygons[i] {
			segs[ln.LeftRightMostAsLine()] = struct{}{}
		}
	}
	var segments []maths.Line
	if extent > 0 {
		segments = []maths.Line{
			{maths.Pt{-10, -10}, maths.Pt{-10, extent + 10}},
			{maths.Pt{-10, -10}, maths.Pt{extent + 10, -10}},
			{maths.Pt{extent + 10, -10}, maths.Pt{extent + 10, extent + 10}},
			{maths.Pt{-10, extent + 10}, maths.Pt{extent + 10, extent + 10}},
		}
	}
	for ln := range segs {
		segments = append(segments, ln)
	}
	if len(segments) <= 1 {
		return nil
	}
	return segments
}

func getTriangles(pts1, pts2 []maths.Pt, pt2MaxY map[maths.Pt]float64) (triangles []maths.Triangle) {

	j := 0
	//log.Printf("Sweep for: \n\tpt1: %#v \n\tpt2: %#v ", pts1, pts2)

LoopI:
	for i := 0; i < len(pts2)-1; i++ {
		for j < len(pts1) {
			maxy, ok := pt2MaxY[pts1[j]]
			if j == len(pts1)-1 {
				triangles = append(triangles, maths.NewTriangle(pts1[j], pts2[i], pts2[i+1]))
				continue LoopI
			}
			if !ok || maxy <= pts2[i].Y {
				triangles = append(triangles, maths.NewTriangle(pts1[j], pts1[j+1], pts2[i]))
				j++
				continue
			}
			triangles = append(triangles, maths.NewTriangle(pts1[j], pts2[i], pts2[i+1]))
			continue LoopI
		}
	}
	return triangles
}

func getTrianglesNodes(hm hitmap.Interface, pts1, pts2 []maths.Pt, pt2MaxY map[maths.Pt]int64) (triangles []*maths.TriangleNode, total int) {

	var triangle maths.Triangle
	var totalTri int
	j := 0
	//log.Printf("Sweep for: \n\tpt1: %#v \n\tpt2: %#v ", pts1, pts2)

	//LoopI:
	for i := 0; i < len(pts2)-1; i++ {
		for j < len(pts1) {
			totalTri++
			maxy, ok := pt2MaxY[pts1[j]]
			if j == len(pts1)-1 {
				triangle = maths.NewTriangle(pts1[j], pts2[i], pts2[i+1])

				/*
					log.Println("Triangle:", "j[", j, "]", pts1[j], "i[", i, "]", pts2[i], "i[", i+1, "]", pts2[i+1])
					log.Printf("\tCenter: %v -- %v",
						triangle.Center(),
						hm.LabelFor(triangle.Center()),
					)
				*/

				if hm.LabelFor(triangle.Center()) == maths.Inside {
					triangles = append(triangles, &maths.TriangleNode{
						Triangle: triangle,
						Label:    maths.Inside,
					})
				}
				break
			}
			if !ok || maxy <= int64(pts2[i].Y*100) {
				triangle = maths.NewTriangle(pts1[j], pts1[j+1], pts2[i])
				/*
					log.Println("Triangle:", "j[", j, "]", pts1[j], "j[", j+1, "]", pts1[j+1], "i[", i, "]", pts2[i])
					log.Printf("\tCenter: %v -- %v",
						triangle.Center(),
						hm.LabelFor(triangle.Center()),
					)
				*/

				if hm.LabelFor(triangle.Center()) == maths.Inside {
					triangles = append(triangles, &maths.TriangleNode{
						Triangle: triangle,
						Label:    maths.Inside,
					})
				}
				j++
				continue
			}
			triangle = maths.NewTriangle(pts1[j], pts2[i], pts2[i+1])

			/*
				log.Println("Triangle:", "j[", j, "]", pts1[j], "i[", i, "]", pts2[i], "i[", i+1, "]", pts2[i+1])
				log.Printf("\tCenter: %v -- %v",
					triangle.Center(),
					hm.LabelFor(triangle.Center()),
				)
			*/
			if hm.LabelFor(triangle.Center()) == maths.Inside {
				triangles = append(triangles, &maths.TriangleNode{
					Triangle: triangle,
					Label:    maths.Inside,
				})
			}
			break
		} // loop for j
	} // loop for i
	//log.Println("X", pts1[0].X, "Total", totalTri, "Inside", len(triangles), "outside", totalTri-len(triangles))
	return triangles, totalTri
}

func destructure3(hm hitmap.Interface, adjustbb float64, segments []maths.Line, extent float64) ([]*maths.TriangleNode, [4]maths.Pt, int) {

	var lines []maths.Line
	adjustbb = 0.0

	// linesToSplit holds a list of points for that segment to be split at. This list will have to be
	// ordered and deuped.
	splitPts := make([][]maths.Pt, len(segments))

	maths.FindIntersects(segments, func(src, dest int, ptfn func() maths.Pt) bool {

		sline, dline := segments[src], segments[dest]

		/*
			pt.X = float64(int64(pt.X))
			pt.Y = float64(int64(pt.Y))
		*/
		// Check to see if the end points of sline and dline intersect?
		if (sline[0].IsEqual(dline[0])) ||
			(sline[0].IsEqual(dline[1])) ||
			(sline[1].IsEqual(dline[0])) ||
			(sline[1].IsEqual(dline[1])) {
			return true
		}

		pt := ptfn().Round() // left most point.
		if !sline.InBetween(pt) || !dline.InBetween(pt) {
			////log.Println("Intersect not on the line.")
			return true
		}
		// log.Println("Intersect point found: ", pt, "src", sline, "dest", dline)

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
				mappts[lrln[0]] = struct{}{}
				mappts[lrln[1]] = struct{}{}
				xs = append(xs, lrln[0].X, lrln[1].X)
				lines = append(lines, lrln)
				lpt = splitPts[i][j]
			}
			if !lpt.IsEqual(rpt) {
				lrln = maths.Line{lpt, rpt}.LeftRightMostAsLine()
				mappts[lrln[0]] = struct{}{}
				mappts[lrln[1]] = struct{}{}
				xs = append(xs, lrln[0].X, lrln[1].X)
				lines = append(lines, lrln)
			}
		}
	}

	sort.Float64s(xs)
	//log.Println("Lines:", lines)
	miny, maxy = miny-adjustbb, maxy+adjustbb
	minx, maxx := xs[0]-adjustbb, xs[len(xs)-1]+adjustbb
	xs = append(append([]float64{minx}, xs...), maxx)
	lx := xs[0]
	uxs = append(uxs, lx)
	//offset := len(lines)
	lines = append(lines, maths.Line{maths.Pt{lx, miny}, maths.Pt{lx, maxy}})
	for _, x := range xs[1:] {
		if x == lx {
			continue
		}
		lines = append(lines,
			/*
				maths.Line{maths.Pt{lx, miny}, maths.Pt{x, miny}},
				maths.Line{maths.Pt{lx, maxy}, maths.Pt{x, maxy}},
			*/
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
		//log.Println("src", sline, "dest", dline, "intersect", pt)
		if !sline.InBetween(pt) || !dline.InBetween(pt) {
			//log.Println("Intersect not on the line.")
			return true
		}
		if !(pt.IsEqual(sline[0]) || pt.IsEqual(sline[1])) {
			//log.Println("Adding pt to src.")
			splitPts[src] = append(splitPts[src], pt)
		}
		if !(pt.IsEqual(dline[0]) || pt.IsEqual(dline[1])) {
			//log.Println("Adding pt to dest.")
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
			//log.Println("MaxY Check for ", pt1, pt2)

			if y1, ok := pt2MaxY[pt1]; !ok || y1 < int64(pt2.Y*100) {
				pt2MaxY[pt1] = int64(pt2.Y * 100)
			}
		}
	}
	{
		//log.Println("Lines:", lines)
		for i := range lines {
			if splitPts[i] == nil {
				// We are not splitting the line.
				//log.Println("Not splitting: ", lines[i])
				add2Maps(lines[i][0], lines[i][1])
				continue
			}

			//log.Println("Going to split: ", lines[i])
			sort.Sort(points.ByXY(splitPts[i]))
			lidx, ridx := lines[i].XYOrderedPtsIdx()
			lpt, rpt := lines[i][lidx], lines[i][ridx]
			for j := range splitPts[i] {
				if lpt.IsEqual(splitPts[i][j]) {
					// Skipp dups.
					continue
				}
				//log.Println("Adding the following points.", lpt, splitPts[i][j])
				add2Maps(lpt, splitPts[i][j])
				lpt = splitPts[i][j]
			}
			if !lpt.IsEqual(rpt) {
				//log.Println("Adding the following points.", lpt, rpt)
				add2Maps(lpt, rpt)
			}
		}
	}

	var trianglesLock sync.Mutex

	var triangles []*maths.TriangleNode
	var totalTri int

	for i := range uxs {
		x2pts[uxs[i]] = points.SortAndUnique(x2pts[uxs[i]])
	}

	var wg sync.WaitGroup
	var idChan = make(chan int)
	var lenuxs = len(uxs) - 1
	// seems to work the best

	var worker = func(id int) {
		var tmp []*maths.TriangleNode
		var tt, total int
		for i := range idChan {

			tmp, tt = getTrianglesNodes(
				hm,
				//hitmap.AllwaysInside,
				x2pts[uxs[i]],
				x2pts[uxs[i+1]],
				pt2MaxY,
			)
			total += tt
			if len(tmp) == 0 {
				continue
			}
			trianglesLock.Lock()
			triangles = append(triangles, tmp...)
			trianglesLock.Unlock()
		}
		trianglesLock.Lock()
		totalTri = totalTri + total
		trianglesLock.Unlock()
		wg.Done()
	}
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go worker(i)
	}

	for i := 0; i < lenuxs; i++ {
		idChan <- i
	}
	/*
		_ = lenuxs
		idChan <- 11
	*/
	close(idChan)
	wg.Wait()
	log.Println("Returning the following triangles:", len(triangles), "of", totalTri)
	/*
		for i := range triangles {
			log.Printf("\t%v:\t%v", i, triangles[i].Triangle)

		}
		panic("stop")
	*/

	return triangles, [4]maths.Pt{
		maths.Pt{minx, miny},
		maths.Pt{minx, maxy},
		maths.Pt{maxx, maxy},
		maths.Pt{maxx, miny},
	}, totalTri

}

type hitMapSeg struct {
	subject []float64
	label   maths.Label // default assume outside.
	bb      [4]float64
}

type HitMap []hitMapSeg

func (hm HitMap) LabelFor(pt maths.Pt) maths.Label {
	for i := len(hm) - 1; i >= 0; i-- {
		// Outside of the bounding box. skip
		if pt.X < hm[i].bb[0] || pt.Y < hm[i].bb[1] || pt.X > hm[i].bb[2] || pt.Y > hm[i].bb[3] {
			continue
		}
		var line maths.Line
		count := 0
		ray := maths.Line{pt, maths.Pt{hm[i].bb[0] - 1, pt.Y}}
		// eliminate segments we don't need touch.
		lxi, lyi := len(hm[i].subject)-2, len(hm[i].subject)-1
		for xi, yi := 0, 1; yi < len(hm[i].subject); xi, yi = xi+2, yi+2 {
			line[0].X = hm[i].subject[lxi]
			line[0].Y = hm[i].subject[lyi]
			line[1].X = hm[i].subject[xi]
			line[1].Y = hm[i].subject[yi]

			deltaY := line[1].Y - line[0].Y
			// If the line is horizontal skipp it.
			if deltaY == 0 {
				continue
			}
			// if both points are greater or equal to the pts x we can remove it.
			if line[0].X >= pt.X && line[1].X >= pt.X {
				continue
			}
			// if the line is above ray, we don't need to consider it.
			if line[0].Y <= pt.Y && line[1].Y <= pt.Y {
				continue
			}
			ray[1].X = line[0].X
			if ray[1].X > line[1].X {
				ray[1].X = line[1].X
			}
			// move the point out by 10
			ray[1].X -= 10

			pt, ok := maths.Intersect(ray, line)
			if !ok || !line.InBetween(pt) || !ray.InBetween(pt) {
				continue
			}

			count++
		}
		// Even means outside, odd means the point is contained.

		if count%2 != 0 {
			return hm[i].label
		}
	}
	//log.Println(pt, "All miss, outside!")
	return maths.Outside
}

func NewHitMap(segs [][]maths.Line) HitMap {
	defer gotrace.T()()
	var hm = make(HitMap, 0, len(segs))
	var pts []maths.Pt
	var setmin = true
	var minx, maxx, miny, maxy float64
	for i := range segs {
		if len(segs[i]) <= 1 {
			// Skip this segment.
			continue
		}
		pts = pts[:0]
		setmin = true
		for j := range segs[i] {
			pts = append(pts, segs[i][j][0])
			if setmin || segs[i][j][0].X > maxx {
				maxx = segs[i][j][0].X
			}
			if setmin || segs[i][j][0].Y > maxy {
				maxy = segs[i][j][0].Y
			}
			if setmin || segs[i][j][0].X < minx {
				minx = segs[i][j][0].X
			}
			if setmin || segs[i][j][0].Y < miny {
				miny = segs[i][j][0].Y
			}
			setmin = false
		}

		if !segs[i][0][0].IsEqual(segs[i][len(segs[i])-1][1]) {
			pts = append(pts, segs[i][len(segs[i])-1][1])
		}
		if len(pts) == 1 {
			// skip single points.
			//log.Println("Skipping because point count is 1.")
			continue
		}
		if maths.AreaOfRing(pts...) == 0 {
			// skip zero area segments.
			//log.Println("Skipping because area zero")
			continue
		}
		hmseg := hitMapSeg{bb: [4]float64{minx, miny, maxx, maxy}}
		for _, pt := range pts {
			hmseg.subject = append(hmseg.subject, pt.X, pt.Y)
		}
		if maths.WindingOrderOf(hmseg.subject) == maths.Clockwise {
			hmseg.label = maths.Inside
		} else {
			hmseg.label = maths.Outside
		}
		hm = append(hm, hmseg)
	}
	//log.Println("HitMap:", hm)
	return hm
}

// We only care about the first triangle node, as an edge can only contain two triangles.
type aNodeList map[maths.Line]*maths.TriangleNode

// AddTrinagleForPts will order the points, create a new Triangle and add it to the Node List.
func (nl aNodeList) AddTriangle(tri *maths.TriangleNode) {

	/*
		tri = &maths.TriangleNode{
			Triangle: triangle,
			Label:    label,
		}
	*/

	edges := tri.LREdges()
	for i := range edges {
		node, ok := nl[edges[i]]
		if !ok {
			//log.Println("Added edge:", edges[i])
			nl[edges[i]] = tri
			continue
		}
		delete(nl, edges[i])
		idx, _ := node.FindEdge(edges[i])
		if idx == -1 {
			//log.Println("Tri:", node, "does not have edge", edges[i], " yet maps says it should.")
			panic("issue")
		}
		//log.Println(edges[i], " : Setting up my[", tri.Triangle, ":", tri.Label, "] neighbor(", i, ") to node[", node.Triangle, "]")
		tri.Neighbors[i].Node = node
		//log.Println(edges[i], " : Setting up nodes[", node.Triangle, ":", node.Label, "] neighbor(", idx, ") to me[", tri.Triangle, "]")
		node.Neighbors[idx].Node = tri

	}
}

func generateTriangleGraph(triangles []*maths.TriangleNode, bbox [4]maths.Pt) *maths.TriangleGraph {
	nl := make(aNodeList)
	for i := range triangles {
		//log.Println("Looking at Triangle(", i, ")[", triangles[i], "]:")
		nl.AddTriangle(triangles[i])
	}
	//log.Println("NL:", nl)
	return maths.NewTriangleGraph(triangles, bbox)
}
func GenerateTriangleGraph(hm hitmap.Interface, adjustbb float64, polygons [][]maths.Line, extent float64) (*maths.TriangleGraph, int) {
	//defer gotrace.T()
	segments := destructure2(polygons, extent)
	if segments == nil {
		return nil, 0
	}
	//hitmap := NewHitMap(polygons)
	triangles, bbox, totalTri := destructure3(hm, adjustbb, segments, extent)
	//return generateTriangleGraph(triangles, bbox), totalTri
	return generateTriangleGraph(triangles, bbox), totalTri
}
func GenerateTriangleGraph1(hm hitmap.Interface, adjustbb float64, polygons [][]maths.Line, extent float64) (TriMP, int) {
	//defer gotrace.T()
	segments := destructure2(polygons, extent)
	if segments == nil {
		return nil, 0
	}
	//	cpolygons, _, totalTri := destructure4(hm, adjustbb, segments, extent)
	cpolygons, _, totalTri := destructure5(hm, extent, segments, extent)
	return cpolygons, totalTri
}
