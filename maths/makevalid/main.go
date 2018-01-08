package makevalid

import (
	"context"
	"errors"
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
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
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

// destructure2 will split the ploygons up and split lines where they intersect. It will also, add a bounding box and a set of lines crossing from the end points of the bounding box to the center.
func destructure2(polygons [][]maths.Line, clipbox *points.Extent) []maths.Line {
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
		segments = append(segments, maths.NewLinesFloat64(edges[:]...)...)
	}
	for ln := range segs {
		segments = append(segments, ln)
	}
	if len(segments) <= 1 {
		return nil
	}
	return segments
}

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

func destructure5(ctx context.Context, hm hitmap.Interface, clipbox *points.Extent, segments []maths.Line) ([][][]maths.Pt, error) {

	var lines []maths.Line

	// linesToSplit holds a list of points for that segment to be split at. This list will have to be
	// ordered and deuped.
	splitPts := make([][]maths.Pt, len(segments))

	maths.FindIntersects(segments, func(src, dest int, ptfn func() maths.Pt) bool {

		if ctx.Err() != nil {
			return true
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
		if !(pt.IsEqual(sline[0]) || pt.IsEqual(sline[1])) {
			splitPts[src] = append(splitPts[src], pt)
		}
		if !(pt.IsEqual(dline[0]) || pt.IsEqual(dline[1])) {
			splitPts[dest] = append(splitPts[dest], pt)
		}
		return true
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var xs []float64
	var uxs []float64
	miny, maxy := segments[0][1].Y, segments[0][1].Y
	{
		mappts := make(map[maths.Pt]struct{}, len(segments)*2)
		var slrln maths.Line
		var lrln [2][2]float64

		for i := range segments {
			////log.Println("Looking at segment", i, segments[i])
			if splitPts[i] == nil {
				slrln = segments[i].LeftRightMostAsLine()
				lrln = [2][2]float64{{slrln[0].X, slrln[0].Y}, {slrln[1].X, slrln[1].Y}}
				if !clipbox.ContainsLine(lrln) {
					// Outside of the clipping area.
					continue
				}
				mappts[slrln[0]] = struct{}{}
				mappts[slrln[1]] = struct{}{}
				//log.Println("Adding to xs:", lrln[0][0], lrln[1][0])
				xs = append(xs, lrln[0][0], lrln[1][0])
				miny = MinF64(miny, lrln[0][1], lrln[1][1])
				maxy = MaxF64(maxy, lrln[0][1], lrln[1][1])

				lines = append(lines, slrln)
				continue
			}
			// Context cancelled.
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			sort.Sort(points.ByXY(splitPts[i]))
			lidx, ridx := maths.Line(segments[i]).XYOrderedPtsIdx()
			lpt, rpt := segments[i][lidx], segments[i][ridx]
			for j := range splitPts[i] {
				if lpt.IsEqual(splitPts[i][j]) {
					// Skipp dups.
					continue
				}
				slrln = maths.Line{lpt, splitPts[i][j]}.LeftRightMostAsLine()
				lrln = [2][2]float64{{slrln[0].X, slrln[0].Y}, {slrln[1].X, slrln[1].Y}}
				lpt = splitPts[i][j]
				if !clipbox.ContainsLine(lrln) {
					// Outside of the clipping area.
					continue
				}
				mappts[slrln[0]] = struct{}{}
				mappts[slrln[1]] = struct{}{}
				//log.Println("Adding to xs:", lrln[0][0], lrln[1][0])
				xs = append(xs, lrln[0][0], lrln[1][0])
				lines = append(lines, slrln)
				// Context cancelled.
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if !lpt.IsEqual(rpt) {
				slrln = maths.Line{lpt, rpt}.LeftRightMostAsLine()
				lrln = [2][2]float64{{slrln[0].X, slrln[0].Y}, {slrln[1].X, slrln[1].Y}}
				if !clipbox.ContainsLine(lrln) {
					// Outside of the clipping area.
					continue
				}
				mappts[slrln[0]] = struct{}{}
				mappts[slrln[1]] = struct{}{}
				//log.Println("Adding to xs:", lrln[0][0], lrln[1][0])
				xs = append(xs, lrln[0][0], lrln[1][0])
				lines = append(lines, slrln)
			}
			// Context cancelled.
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
	}
	// Context cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
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
	// Context cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	splitPts = make([][]maths.Pt, len(lines))

	maths.FindIntersects(lines, func(src, dest int, ptfn func() maths.Pt) bool {

		// Context cancelled.
		if ctx.Err() != nil {
			return true
		}
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
	// Context cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
			// Context cancelled.
			if err := ctx.Err(); err != nil {
				return nil, err
			}

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

	// Context cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	for i := range uxs {
		x2pts[uxs[i]] = points.SortAndUnique(x2pts[uxs[i]])
	}

	// Context cancelled.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var idChan = make(chan int)
	var lenuxs = len(uxs) - 1

	var ringCols = make([]plyg.RingCol, lenuxs)

	var worker = func(id int, ctx context.Context) {
		for i := range idChan {
			ringCols[i] = plyg.BuildRingCol(
				ctx,
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
		go worker(i, ctx)
	}
	for i := 0; i < lenuxs; i++ {
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

	plygs := plyg.GenerateMultiPolygon(ringCols)
	return plygs, nil
}

func MakeValid(ctx context.Context, hm hitmap.Interface, extent *points.Extent, plygs ...[]maths.Line) (polygons [][][]maths.Pt, err error) {

	segments := destructure2(insureConnected(plygs...), extent)
	if segments == nil {
		return nil, nil
	}
	return destructure5(ctx, hm, extent, segments)
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
