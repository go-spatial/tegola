package makevalid

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sort"
	"sync"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/makevalid/plyg"
	"github.com/terranodo/tegola/maths/points"
)

var numWorkers = 1

func init() {
	numWorkers = runtime.NumCPU()
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

/*
func MaxF64(vals ...float64) (max float64) {
	if len(vals) == 0 {
		return 0
	}
	max = vals[0]
	for _, f := range vals[1:] {
		if f > max {
			max = f
		}
	}
	return max
}
func MinF64(vals ...float64) (min float64) {
	if len(vals) == 0 {
		return 0
	}
	min = vals[0]
	for _, f := range vals[1:] {
		if f < min {
			min = f
		}
	}
	return min
}
*/

// destructure2  splits the polygon into a set of segements adding the segments of the clipbox as well.
func destructure2(polygons [][]maths.Line, clipbox *points.Extent) []maths.Line {
	// First we need to combine all the segments.
	segs := make(map[maths.Line]struct{})
	for i := range polygons {
		for _, ln := range polygons[i] {
			segs[ln.LeftRightMostAsLine()] = struct{}{}
		}
	}

	var segments []maths.Line
	// Add the clipbox segments to the set of segments.
	if clipbox != nil {
		edges := clipbox.LREdges()
		lns := maths.NewLinesFloat64(edges[:]...)
		for i := range lns {
			segs[lns[i]] = struct{}{}
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

func logOutBuildRings(pt2maxy map[maths.Pt]int64, xs []float64, x2pts map[float64][]maths.Pt) (output string) {
	if debug {
		output = fmt.Sprintf("xs := %#v\n", xs)
		output += fmt.Sprintf("x2pts := %#v\n", x2pts)
		output += fmt.Sprintf("Pt2MaxY := %#v\n", pt2maxy)
		output += fmt.Sprintf("Cols := []struct{ idx int,  col1 []maths.Pt, col2 []maths.Pt}{")
		for i := 0; i < len(xs)-1; i++ {
			output += fmt.Sprintf("{idx: %[1]v, col1: %v, col2: %v}, ", i, x2pts[xs[i]], x2pts[xs[i+1]])
		}
		output += "}\n"
	}
	return output
}

func destructure5(ctx context.Context, hm hitmap.Interface, cpbx *points.Extent, plygs [][]maths.Line) ([][][]maths.Pt, error) {

	if len(plygs) == 0 {
		return nil, nil
	}

	// Make copy because we are going to modify the clipbox.
	clipbox := _adjustClipBox(cpbx, plygs)
	// Just trying to clip a polygon that is on the border.
	if clipbox[0][0] == clipbox[1][0] || clipbox[0][1] == clipbox[1][1] {
		if debug {
			log.Println("clip area too small: Clipbox:", cpbx)
		}
		return nil, nil
	}

	segments := destructure2(plygs, clipbox)
	if segments == nil {
		return nil, nil
	}

	var lines []maths.Line
	if debug {
		log.Println("Destructure5 called.")
		defer func() {
			if debug {
				log.Println("Destructure5 ended.")
			}
		}()
		log.Printf("segments /*(%v)*/ := %#v", len(segments), segments)
		log.Printf("clipbox := %#v", clipbox)
	}

	flines, err := splitSegments(ctx, segments, clipbox)
	if err != nil {
		return nil, err
	}

	if debug {
		log.Printf("flines := %#v", flines)
	}

	pts := allPointsForSegments(flines)
	xs := sortUniqueF64(allCoordForPts(0, pts...))

	// Add lines at each x going from the miny to maxy.
	for i := range xs {
		flines = append(flines, [2][2]float64{{xs[i], clipbox[0][1]}, {xs[i], clipbox[1][1]}})
	}

	lines = maths.NewLinesFloat64(flines...)
	splitPts, err := splitPoints(ctx, lines)
	if err != nil {
		return nil, err
	}

	// The split points should now include the columns. We need to associate
	// each point with the value of their x, and the max y value to the next
	// x.

	x2pts := make(map[float64][]maths.Pt)
	pt2MaxY := make(map[maths.Pt]int64)
	clipMaxY100 := int64(clipbox[1][1] * 100)
	maxY100Val := func(y float64) int64 {
		y100 := int64(y * 100)
		if y < clipbox[1][1] {
			return y100
		}
		return clipMaxY100
	}
	var add2Maps = func(pt1, pt2 maths.Pt) {
		x2pts[pt1.X] = append(x2pts[pt1.X], pt1)
		x2pts[pt2.X] = append(x2pts[pt2.X], pt2)
		if pt2.X != pt1.X {
			y1, ok := pt2MaxY[pt1]
			y100 := maxY100Val(pt2.Y)

			if !ok || y1 < y100 {
				pt2MaxY[pt1] = y100
			}
		}
	}
	for i := range splitPts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := 1; j < len(splitPts[i]); j++ {
			add2Maps(splitPts[i][j-1], splitPts[i][j])
		}
	}

	// Remove any duplicate points.
	for i := range xs {
		x2pts[xs[i]] = points.SortAndUnique(x2pts[xs[i]])
	}

	// Context cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var idChan = make(chan int)
	var lenxs = len(xs) - 1

	if lenxs <= 0 {
		return nil, nil
	}
	var ringCols = make([]plyg.RingCol, lenxs)
	if debug {
		logout := fmt.Sprintf("clipbox := %v\n", clipbox)
		logout += fmt.Sprintf("plygs := %v\n", plygs)
		logout += logOutBuildRings(pt2MaxY, xs, x2pts)
		log.Println("Going to buld out rings:", logout)
	}

	var worker = func(id int, ctx context.Context) {
		var cancelled bool
		for i := range idChan {
			if cancelled {
				continue
			}
			if clipbox != nil {
				if xs[i] < clipbox[0][0] || xs[i] > clipbox[1][0] {
					// Skip working on this one.
					continue
				}
				if xs[i+1] > clipbox[1][0] {
					continue
				}
			}
			ringCols[i], err = plyg.BuildRingCol(
				ctx,
				hm,
				x2pts[xs[i]],
				x2pts[xs[i+1]],
				pt2MaxY,
			)
			if err != nil {
				switch err {
				case context.Canceled:
					cancelled = true
				default:
					if debug {
						logout := fmt.Sprintf("clipbox := %v\n", clipbox)
						logout += fmt.Sprintf("plygs := %v\n", plygs)
						logout += logOutBuildRings(pt2MaxY, xs, x2pts)
						log.Println(logout+"For ", i, "Got error (", err, ") trying to process ")
					}
					//panic(err)
				}
			}

		}
		wg.Done()
	}
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go worker(i, ctx)
	}
	for i := 0; i < lenxs; i++ {
		select {
		case <-ctx.Done():
		case idChan <- i:
			// NoOp
		}
	}

	close(idChan)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err

	}

	ploygs := plyg.GenerateMultiPolygon(ringCols)
	return ploygs, nil
}

func MakeValid(ctx context.Context, hm hitmap.Interface, extent *points.Extent, plygs ...[]maths.Line) (polygons [][][]maths.Pt, err error) {
	return destructure5(ctx, hm, extent, insureConnected(plygs...))
}

type byArea []*ring

func (r byArea) Less(i, j int) bool {
	iarea, _ := r[i].BBArea()
	jarea, _ := r[j].BBArea()
	return iarea < jarea
}
func (r byArea) Len() int {
	return len(r)
}
func (r byArea) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type plygByFirstPt [][][]maths.Pt

func (p plygByFirstPt) Less(i, j int) bool {
	p1 := p[i]
	p2 := p[j]
	if len(p1) == 0 && len(p2) != 0 {
		return true
	}
	if len(p1) == 0 || len(p2) == 0 {
		return false
	}

	if len(p1[0]) == 0 && len(p2[0]) != 0 {
		return true
	}
	if len(p1[0]) == 0 || len(p2[0]) == 0 {
		return false
	}
	return maths.XYOrder(p1[0][0], p2[0][0]) == -1
}

func (p plygByFirstPt) Len() int {
	return len(p)
}
func (p plygByFirstPt) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type ring struct {
	r      []maths.Pt
	end    int
	Closed bool
	hmi    bool
	mi     int
	hbb    bool
	bb     [4]float64
}

func (r *ring) Add(ptslist []maths.Pt) (added bool) {
	if r == nil {
		return false
	}

	if r.Closed {
		return false
	}

	if len(r.r) == 0 {
		r.r = ptslist
		r.end = len(r.r) - 1
		return true
	}

	pend := len(ptslist) - 1

	var ss, ee = r.r[0].IsEqual(ptslist[0]), r.r[r.end].IsEqual(ptslist[pend])
	var es, se = r.r[r.end].IsEqual(ptslist[0]), r.r[0].IsEqual(ptslist[pend])

	// First check to see if both end points match.
	if ss && ee {
		if pend-1 > 1 {
			r.r = append(r.r, points.Reverse(ptslist[1:pend-1])...)
		}
		r.end = len(r.r) - 1
		r.Closed = true
		return true
	}
	if es && se {
		if pend-1 > 1 {
			r.r = append(r.r, ptslist[1:pend-1]...)
		}
		r.end = len(r.r) - 1
		r.Closed = true
		return true
	}

	if ss {
		r.r = append(points.Reverse(ptslist[1:]), r.r...)
		r.end = len(r.r) - 1
		return true
	}
	if se {
		r.r = append(ptslist[:pend], r.r...)
		r.end = len(r.r) - 1
		return true
	}
	if es {
		r.r = append(r.r, ptslist[1:]...)
		r.end = len(r.r) - 1
		return true
	}
	if ee {
		r.r = append(r.r, points.Reverse(ptslist[:pend])...)
		r.end = len(r.r) - 1
		return true
	}
	return false

}

func (r *ring) Simplify() {
	if r == nil || len(r.r) == 0 {
		return
	}
	sring := make([]maths.Pt, 0, len(r.r))
	lpt := r.end
	r.bb = [4]float64{r.r[lpt].X, r.r[lpt].Y, r.r[lpt].X, r.r[lpt].Y}
	r.hbb = true
	for j := 0; j < r.end; j++ {
		lln := maths.Line{r.r[lpt], r.r[j+1]}
		cln := maths.Line{r.r[j], r.r[j+1]}
		m1, _, ok1 := lln.SlopeIntercept()
		m2, _, ok2 := cln.SlopeIntercept()
		if ok1 == ok2 && m1 == m2 {
			continue
		}
		sring = append(sring, r.r[lpt])
		// Update bb values.
		if r.r[lpt].X < r.bb[0] {
			r.bb[0] = r.r[lpt].X
		}
		if r.r[lpt].Y < r.bb[1] {
			r.bb[1] = r.r[lpt].Y
		}
		if r.r[lpt].X > r.bb[2] {
			r.bb[2] = r.r[lpt].X
		}
		if r.r[lpt].Y > r.bb[3] {
			r.bb[3] = r.r[lpt].Y
		}
		lpt = j
	}
	// Add the second to last element.
	sring = append(sring, r.r[lpt])
	if len(sring) < 4 {
		// Don't do anything.
		return
	}
	if sring[0].IsEqual(sring[len(sring)-1]) {
		sring = sring[0 : len(sring)-1]
	}
	r.r = sring
	r.hmi = false
	r.end = len(r.r) - 1
}

func (r *ring) BBox() ([4]float64, error) {
	if r == nil {
		return [4]float64{}, errors.New("r is nil")
	}
	if len(r.r) == 0 {
		return [4]float64{}, errors.New("r.r is nil ")
	}
	if r.hbb {
		return r.bb, nil
	}
	r.hbb = true
	r.bb = [4]float64{r.r[0].X, r.r[0].Y, r.r[0].X, r.r[0].Y}
	for _, pt := range r.r[1:] {
		if pt.X < r.bb[0] {
			r.bb[0] = pt.X
		}
		if pt.Y < r.bb[1] {
			r.bb[1] = pt.Y
		}
		if pt.X > r.bb[2] {
			r.bb[2] = pt.X
		}
		if pt.Y > r.bb[3] {
			r.bb[3] = pt.Y
		}
	}
	return r.bb, nil
}
func (r *ring) BBArea() (a float64, err error) {
	bb, err := r.BBox()
	if err != nil {
		return 0, err
	}
	return (bb[2] - bb[0]) * (bb[3] - bb[1]), nil
}

func (r *ring) ReOrder() {
	if r == nil || len(r.r) == 0 {
		return
	}
	if r.hmi {
		if r.mi == 0 {
			return
		}
		r.r = append(r.r[r.mi:], r.r[:r.mi]...)
		r.mi = 0
		return
	}
	var minidx = 0
	for j := 1; j < len(r.r); j++ {
		if maths.XYOrder(r.r[minidx], r.r[j]) == 1 {
			minidx = j
		}
	}

	if minidx == 0 {
		return
	}
	r.r = append(r.r[minidx:], r.r[:minidx]...)
	r.hmi = true
	r.mi = 0
}

func newRing(pts []maths.Pt) *ring {
	if len(pts) == 0 {
		return &ring{}
	}
	return &ring{
		r:   pts,
		end: len(pts) - 1,
	}
}

func constructPolygon(lns []maths.Line) (rpts [][]maths.Pt) {

	lines := make([]maths.Line, len(lns))
	for i := range lns {
		lines[i] = maths.Line{
			maths.Pt{
				float64(int64(lns[i][0].X)),
				float64(int64(lns[i][0].Y)),
			},
			maths.Pt{
				float64(int64(lns[i][1].X)),
				float64(int64(lns[i][1].Y)),
			},
		}
	}

	// We sort the lines, for a couple of reasons.
	// The first is because the smallest and largest lines are going to be part of the external ring.
	// The second by sorting it moves lines that are connected closer together.
	sort.Sort(maths.ByXYLine(lines))

	var rings []*ring

NextLine:
	for l := range lines {
		for r := range rings {
			if rings[r] == nil || rings[r].Closed {
				continue
			}
			if rings[r].Add(lines[l][:]) {
				continue NextLine
			}
		}
		// Need to add it to a new ring.
		rings = append(rings, newRing(lines[l][:]))
	}

	// Time to loop through the rings and see if any of them should be attached together.
	for i := range rings[:len(rings)-1] {
		// Only care about the open rings.
		if rings[i] == nil || rings[i].Closed {
			continue
		}
		for j := range rings[i+1:] {
			idx := j + i + 1
			if rings[idx] == nil {
				continue
			}
			if rings[i].Add(rings[idx].r) {
				// Close out the ring because it got added to ring[i]
				rings[idx] = nil
			}
		}
	}

	// Need to simplify rings.

	for _, ring := range rings {
		ring.Simplify()
		ring.ReOrder()
	}
	// Need to sort the rings by size. The largest ring by area needs to be the first ring.

	sort.Sort(byArea(rings))
	for i := range rings {
		if rings[i] == nil {
			continue
		}
		rpts = append(rpts, rings[i].r)
	}
	return rpts
}

type dedupinp struct {
	A    []maths.Pt
	idxs []int
}

func (d dedupinp) Len() int           { return len(d.A) }
func (d dedupinp) Less(i, j int) bool { return maths.XYOrder(d.A[d.idxs[i]], d.A[d.idxs[j]]) == -1 }
func (d dedupinp) Swap(i, j int)      { d.idxs[i], d.idxs[j] = d.idxs[j], d.idxs[i] }
func (d dedupinp) Dedup() {
	d.idxs = make([]int, len(d.A))
	for i := range d.idxs {
		d.idxs[i] = i
	}
	sort.Sort(d)

	// Find the dups.
	ba := make([][2]int, 0, len(d.A)/2)
	var b [2]int
	for i := 1; i < len(d.A); i++ {
		if d.A[d.idxs[i]] == d.A[d.idxs[i-1]] {
			b[1] = i
			continue
		}
		if b[1] != 0 {
			ba = append(ba, b)
			b[1] = 0
		}
		b[0] = i
	}
	if b[1] != 0 {
		ba = append(ba, b)
	}

	// ba has the dups. Need to remove them from the index array.
	for i := len(ba) - 1; i >= 0; i-- {
		switch {
		case ba[i][0] == 0:
			d.idxs = d.idxs[ba[i][1]:]
		case ba[i][1] == len(d.A)-1:
			d.idxs = d.idxs[:ba[i][0]+1]
		default:
			d.idxs = append(d.idxs[:ba[i][0]], d.idxs[ba[i][1]:]...)
		}
	}
	sort.Ints(d.idxs)

}

func dedup(a []maths.Pt) {
	var idxs = make([]int, len(a))
	for i := range idxs {
		idxs[i] = i
	}

}

func fixup(pts [][]maths.Pt) {
	if len(pts) == 0 {
		return
	}

	if maths.WindingOrderOfPts(pts[0]) == maths.CounterClockwise {
		points.Reverse(pts[0])
	}
	if len(pts) == 1 {
		return
	}
	for i := range pts[1:] {
		if maths.WindingOrderOfPts(pts[i+1]) == maths.Clockwise {
			points.Reverse(pts[i+1])
		}
	}

}
