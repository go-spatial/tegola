package edgemap

import (
	"log"
	"sort"
	"sync"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap/plyg"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

type semipolygon struct {
	polygon [2][]maths.Pt
	label   maths.Label
}

func (sp *semipolygon) LabelPolygon() (lp LabeledPolygon) {
	if sp == nil {
		return lp
	}
	lp.Label = sp.label
	if len(sp.polygon[0]) == 0 || len(sp.polygon[1]) == 0 {
		return lp
	}
	lp.x1 = sp.polygon[0][0].X
	lp.x1 = sp.polygon[1][0].X
	lp.Pts = append(lp.Pts, sp.polygon[0][len(sp.polygon[0])-1])
	for i := len(sp.polygon[0]) - 2; i >= 0; i-- {
		l := len(lp.Pts) - 1
		// skip dups.
		if lp.Pts[l].IsEqual(sp.polygon[0][i]) {
			continue
		}
		lp.Pts = append(lp.Pts, sp.polygon[0][i])
	}
	for i := range sp.polygon[1] {
		l := len(lp.Pts) - 1
		if lp.Pts[l].IsEqual(sp.polygon[1][i]) {
			continue
		}
		lp.Pts = append(lp.Pts, sp.polygon[1][i])
	}
	var wantOrder maths.WindingOrder
	if lp.Label == maths.Inside {
		wantOrder = maths.Clockwise
	} else {
		wantOrder = maths.CounterClockwise
	}

	if maths.WindingOrderOfPts(lp.Pts) != wantOrder {
		points.Reverse(lp.Pts)
	}

	return lp
}

func (sp *semipolygon) AddToSet(l maths.Label, jpts, ipts []maths.Pt) (lp LabeledPolygon, new bool) {
	if sp == nil {
		return lp, new
	}
	if sp.label != l {
		// if there is something to return
		new = len(sp.polygon[0]) != 0 && len(sp.polygon[1]) != 0
		if new {
			// need to get the polygon,
			lp = sp.LabelPolygon()
		}
		sp.label = l
		sp.polygon[0] = jpts
		sp.polygon[1] = ipts
		return lp, new
	}
	sp.polygon[0] = append(sp.polygon[0], jpts...)
	sp.polygon[1] = append(sp.polygon[1], ipts...)
	return lp, new
}

type LabeledPolygon struct {
	Pts    []maths.Pt
	Label  maths.Label
	x1, x2 float64
}

func getLabeledPolygons(hm hitmap.Interface, pts1, pts2 []maths.Pt, pt2MaxY map[maths.Pt]int64) (lpolygons []LabeledPolygon, total int) {

	var lp LabeledPolygon
	var sp semipolygon
	var label maths.Label
	var triangle maths.Triangle
	var new bool
	j := 0

	//LoopI:
	for i := 0; i < len(pts2)-1; i++ {
		for j < len(pts1) {
			total++
			maxy, ok := pt2MaxY[pts1[j]]
			if j == len(pts1)-1 {
				triangle[0], triangle[1], triangle[2] = pts1[j], pts2[i], pts2[i+1]
				label = hm.LabelFor(triangle.Center())
				if lp, new = sp.AddToSet(
					label,
					[]maths.Pt{pts1[j]},
					[]maths.Pt{pts2[i], pts2[i+1]},
				); new {
					lpolygons = append(lpolygons, lp)
				}
				break
			}
			if !ok || maxy <= int64(pts2[i].Y*100) {
				triangle[0], triangle[1], triangle[2] = pts1[j], pts1[j+1], pts2[i]
				label = hm.LabelFor(triangle.Center())
				if lp, new = sp.AddToSet(
					label,
					[]maths.Pt{pts1[j], pts1[j+1]},
					[]maths.Pt{pts2[i]},
				); new {
					lpolygons = append(lpolygons, lp)
				}
				j++
				continue
			}
			triangle[0], triangle[1], triangle[2] = pts1[j], pts2[i], pts2[i+1]
			label = hm.LabelFor(triangle.Center())
			if lp, new = sp.AddToSet(
				label,
				[]maths.Pt{pts1[j]},
				[]maths.Pt{pts2[i], pts2[i+1]},
			); new {
				lpolygons = append(lpolygons, lp)
			}
			break
		} // loop for j
	} // loop for i
	return lpolygons, total
}

type ColLabeledPolygon []LabeledPolygon

func (clp ColLabeledPolygon) FindFirst(pt maths.Pt, label maths.Label) (pidx, ptidx int) {
	for i := range clp {
		if clp[i].Label != label {
			continue
		}
		for j := range clp[i].Pts {
			if pt.IsEqual(clp[i].Pts[j]) {
				return i, j
			}
		}
	}
	return -1, -1
}

type ColLabeledPolygons []ColLabeledPolygon

func (clps *ColLabeledPolygons) Combine(idx int) bool {
	var lpc []LabeledPolygon

	if len((*clps)[idx]) == 0 {
		return false
	}
	/*
		if len((*clps)[idx+1]) == 0 {
			(*clps)[idx+1] = nil
			return true
		}
	*/
	var lineOfContention = (*clps)[idx][0].x2
	var seen = make(map[[2]int]bool)

	var nextIdx = func(cidx int) int {
		if cidx == idx {
			return idx + 1
		}
		return idx
	}

	var walkPly = func(currentIdx int, j int) {
		currentPtIdx := 0
		currentPly := (*clps)[currentIdx][j]
		ply := LabeledPolygon{
			x1:    (*clps)[idx][0].x1,
			Label: currentPly.Label,
		}
		if len((*clps)[idx+1]) != 0 {
			ply.x2 = (*clps)[idx+1][0].x2
		} else {
			ply.x2 = (*clps)[idx][0].x2
		}
		var nextPtIdx, otherIdx int
		var pt maths.Pt

		c := 0
		for {
			//		log.Println("Pts:", currentPly.Pts)
			pt = currentPly.Pts[currentPtIdx]
			//		log.Println("For CPtIdx", currentPtIdx, "getting pt", pt)
			if len(ply.Pts) > 0 && ply.Pts[0].IsEqual(pt) {
				// found a new Polygon.
				//		log.Println(idx, "Adding ply: ", ply, "\n", len(ply.Pts))
				lpc = append(lpc, ply)
				return
			}
			//	log.Println(idx, "Adding pt to ply\n\t", ply.Pts, "\n\t", len(ply.Pts))

			ply.Pts = append(ply.Pts, pt)
			//	log.Println(idx, "After Adding pt to ply\n\t", ply.Pts, "\n\t", len(ply.Pts))
			// is pt on share boundry.
			if pt.X == lineOfContention {
				otheridx := nextIdx(currentIdx)
				j, nextPtIdx = (*clps)[otherIdx].FindFirst(pt, ply.Label)
				// We found a ply on the other columns, need to walk it.
				if nextPtIdx != -1 {
					currentPtIdx = nextPtIdx
					currentIdx = otheridx
					currentPly = (*clps)[otherIdx][j]
					seen[[2]int{otherIdx, nextPtIdx}] = true
				}
			}
			currentPtIdx++
			if len(currentPly.Pts) <= currentPtIdx {
				currentPtIdx = 0
				c++
				if c == 100 {
					log.Println("lpc", lpc, "ply", ply)
					panic("Infinite loop?")
				}
			}
		}
	}

	for i := range (*clps)[idx] {
		// Already see this polygon
		if seen[[2]int{idx, i}] {
			continue
		}
		seen[[2]int{idx, i}] = true
		walkPly(idx, i)
	}
	for i := range (*clps)[idx+1] {
		// Already see this polygon
		if seen[[2]int{idx + 1, i}] {
			continue
		}
		seen[[2]int{idx + 1, i}] = true
		walkPly(idx+1, i)
	}
	(*clps)[idx] = lpc
	(*clps)[idx+1] = nil
	return true
}

func (clps ColLabeledPolygons) TrianglesAsMP() (mp [][][]maths.Pt) {
	for i := range clps {
		for j, clp := range clps[i] {
			if clp.Label != maths.Inside || len(clp.Pts) == 0 {
				continue
			}
			mp = append(mp, [][]maths.Pt{clps[i][j].Pts})
		}
	}
	return mp
}

func destructure4(hm hitmap.Interface, adjustbb float64, segments []maths.Line, extent float64) (ColLabeledPolygons, [4]maths.Pt, int) {

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

	var pLock sync.Mutex

	var totalPoly int

	for i := range uxs {
		x2pts[uxs[i]] = points.SortAndUnique(x2pts[uxs[i]])
	}

	var wg sync.WaitGroup
	var idChan = make(chan int)
	var lenuxs = len(uxs) - 1

	var polygons = make(ColLabeledPolygons, lenuxs)

	var worker = func(id int) {
		var tmp []LabeledPolygon
		var tt, total int
		for i := range idChan {

			tmp, tt = getLabeledPolygons(
				hm,
				x2pts[uxs[i]],
				x2pts[uxs[i+1]],
				pt2MaxY,
			)
			total += tt
			if len(tmp) == 0 {
				continue
			}
			polygons[i] = append(polygons[i], tmp...)
		}
		pLock.Lock()
		totalPoly = totalPoly + total
		pLock.Unlock()
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

	loop := 0
	var removeidx = make(map[int]struct{})
	worker1 := func(id int, idChan chan int) {
		var remove = make([]int, 0, 1024)
		for i := range idChan {
			if polygons.Combine(i) {
				remove = append(remove, i+1)
			}
		}
		pLock.Lock()
		for _, i := range remove {
			removeidx[i] = struct{}{}
		}
		pLock.Unlock()

		wg.Done()
	}
LoopHere:

	idChan = make(chan int)
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go worker1(i, idChan)
	}
	for i := 0; i+1 < len(polygons); {
		if len(polygons[i]) == 0 {
			// skip this one for now.
			pLock.Lock()
			removeidx[i] = struct{}{}
			pLock.Unlock()
			i++
			continue
		}
		idChan <- i
		i += 2
	}
	//	idChan <- 4290
	close(idChan)
	wg.Wait()
	var np ColLabeledPolygons
	if len(removeidx) == 0 {
		log.Println("Exit early!", loop, len(polygons))
		goto ReturnStuff
	}

	for i := range polygons {
		if _, ok := removeidx[i]; ok {
			delete(removeidx, i)
			continue
		}
		np = append(np, polygons[i])
	}
	polygons = np
	loop++
	if loop != 1000 {
		goto LoopHere
	}
ReturnStuff:

	return polygons, [4]maths.Pt{
		maths.Pt{minx, miny},
		maths.Pt{minx, maxy},
		maths.Pt{maxx, maxy},
		maths.Pt{maxx, miny},
	}, totalPoly
}

type TriMP [][][]maths.Pt

func (t TriMP) TrianglesAsMP() [][][]maths.Pt { return t }

func destructure5(hm hitmap.Interface, adjustbb float64, segments []maths.Line, extent float64) (TriMP, [4]maths.Pt, int) {

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
	clipBox := points.BoundingBox{-10, -10, extent + 10, extent + 10}

	var xs []float64
	var uxs []float64
	miny, maxy := segments[0][1].Y, segments[0][1].Y
	{
		mappts := make(map[maths.Pt]struct{}, len(segments)*2)
		var lrln maths.Line

		for i := range segments {
			if splitPts[i] == nil {
				lrln = segments[i].LeftRightMostAsLine()
				if !clipBox.ContainsLine(lrln) {
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
				if !clipBox.ContainsLine(lrln) {
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
				if !clipBox.ContainsLine(lrln) {
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
	return TriMP(plygs), [4]maths.Pt{
		maths.Pt{minx, miny},
		maths.Pt{minx, maxy},
		maths.Pt{maxx, maxy},
		maths.Pt{maxx, miny},
	}, 0
}
