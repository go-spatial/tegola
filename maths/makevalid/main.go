package makevalid

import (
	"errors"
	"log"
	"sort"
	"time"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

func init() {
	log.Println("Using seperated MakeValid.")
}

func trace(msg string) func() {
	tracer := time.Now()
	log.Println(msg, "started at:", tracer)
	return func() {
		etracer := time.Now()
		log.Println(msg, "ElapsedTime in seconds:", etracer.Sub(tracer))
	}
}

const adjustBBoxBy = 1

func MakeValid(hm hitmap.Interface, extent float64, plygs ...[]maths.Line) (polygons [][][]maths.Pt, err error) {
	return makeValid(hm, extent, plygs...)
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
	//insureCorrectWindingOrder(rpts)
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
		//log.Println("Need to flip the winding order to clockwise.")
		points.Reverse(pts[0])
	}
	if len(pts) == 1 {
		return
	}
	for i := range pts[1:] {
		if maths.WindingOrderOfPts(pts[i+1]) == maths.Clockwise {
			//log.Println("Need to flip the winding order counterclockwise.", i+1)
			points.Reverse(pts[i+1])
		}
	}

}

/*
type PointNode struct {
	Pt   maths.Pt
	Next *PointNode
}
type PointList struct {
	Head       *PointNode
	Tail       *PointNode
	isComplete bool
}

func (pl PointList) AsRing() (r Ring) {
	node := pl.Head
	for node != nil {
		r = append(r, node.Pt)
		node = node.Next
	}
	return r
}

func (pl *PointList) IsComplete() bool {
	if pl == nil {
		return false
	}
	return pl.isComplete
}

func (pl *PointList) TryAddLine(l Line) (ok bool) {
	// If a PointList is complete we do not add more lines.
	if pl.isComplete {
		return false
	}

	switch {
	case (l[0].IsEqual(pl.Head.Pt) && l[1].IsEqual(pl.Tail.Pt)) ||
		(l[1].IsEqual(pl.Head.Pt) && l[0].IsEqual(pl.Tail.Pt)):
		pl.isComplete = true
		return true
	case l[0].IsEqual(pl.Head.Pt):
		head := PointNode{
			Pt:   l[1],
			Next: pl.Head,
		}
		pl.Head = &head
		return true
	case l[1].IsEqual(pl.Head.Pt):
		head := PointNode{
			Pt:   l[0],
			Next: pl.Head,
		}
		pl.Head = &head
		return true
	case l[0].IsEqual(pl.Tail.Pt):
		tail := PointNode{Pt: l[1]}
		pl.Tail.Next = &tail
		pl.Tail = &tail
		return true
	case l[1].IsEqual(pl.Tail.Pt):
		tail := PointNode{Pt: l[0]}
		pl.Tail.Next = &tail
		pl.Tail = &tail
		return true
	default:
		return false
	}
}
func NewPointList(line Line) PointList {

	tail := &PointNode{Pt: line[1]}
	head := &PointNode{
		Pt:   line[0],
		Next: tail,
	}
	return PointList{
		Head: head,
		Tail: tail,
	}
}
*/
