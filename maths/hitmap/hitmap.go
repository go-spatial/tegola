package hitmap

import (
	"sort"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

type Interface interface {
	LabelFor(pt maths.Pt) maths.Label
}

type allwaysInside struct{}

func (ai allwaysInside) LabelFor(_ maths.Pt) maths.Label { return maths.Inside }

var AllwaysInside allwaysInside

type bbox struct {
	f    [4]float64
	init bool
}

func (bb *bbox) Contains(pt maths.Pt) bool {
	return pt.X >= bb.f[0] && pt.Y >= bb.f[1] && pt.X <= bb.f[2] && pt.Y <= bb.f[3]
}

func (bb *bbox) Add(pts ...maths.Pt) {
	if bb == nil {
		return
	}
	for _, pt := range pts {
		if !bb.init {
			bb.f = [4]float64{pt.X, pt.Y, pt.X, pt.Y}
			bb.init = true
			return
		}
		if bb.f[0] > pt.X {
			bb.f[0] = pt.X
		}
		if bb.f[1] > pt.Y {
			bb.f[1] = pt.Y
		}
		if bb.f[2] < pt.X {
			bb.f[2] = pt.X
		}
		if bb.f[3] < pt.Y {
			bb.f[3] = pt.Y
		}
	}
}
func (bb *bbox) Coords() [4]float64 {
	if bb == nil {
		return [4]float64{}
	}
	return bb.f
}

type segEvent struct {
	x1         float64
	y1         int64
	x2         float64
	y2         int64
	m          float64
	b          float64
	isMDefined bool
}

type segEvents []segEvent

func (se segEvents) Len() int { return len(se) }
func (se segEvents) Less(i, j int) bool {
	if se[i].x1 == se[j].x1 {
		return se[i].y1 < se[j].y1
	}
	return se[i].x1 < se[j].x1
}
func (se segEvents) Swap(i, j int) { se[i], se[j] = se[j], se[i] }
func (se *segEvents) Add(l maths.Line) {
	if se == nil {
		return
	}
	// Skip dup points
	var ev segEvent
	if l[0].IsEqual(l[1]) {
		return
	}
	switch {
	case l[0].X == l[1].X && l[0].Y > l[1].Y:
		fallthrough
	case l[0].X < l[1].X:
		ev.x1 = l[0].X
		ev.y1 = int64(l[0].Y * 100)
		ev.x2 = l[1].X
		ev.y2 = int64(l[1].Y * 100)
	default:
		ev.x1 = l[1].X
		ev.y1 = int64(l[1].Y * 100)
		ev.x2 = l[0].X
		ev.y2 = int64(l[0].Y * 100)
	}
	ev.m, ev.b, ev.isMDefined = l.SlopeIntercept()
	*se = append(*se, ev)
}

func (se segEvents) Contains(pt maths.Pt) (ok bool) {
	var i, count int
	var y, uy, ly int64
	var y1100, y2100 int64
	var y100 = int64(pt.Y * 100)
	for i = 0; i < len(se) && se[i].x1 <= pt.X; i++ {
		if se[i].y1 <= se[i].y2 {
			uy, ly = se[i].y1, se[i].y2
		} else {
			uy, ly = se[i].y2, se[i].y1
		}

		if y100 < uy || y100 > ly {
			continue
		}

		//y1100, y2100 = int64(seg.events[i].y1*100), int64(seg.events[i].y2*100)
		y1100, y2100 = se[i].y1, se[i].y2

		// Horizontal line
		if y1100 == y2100 {
			if y100 == y1100 {
				if se[i].x1 <= pt.X &&
					pt.X <= se[i].x2 {
					// if we are on the line return true.
					return true
				}
				continue
			}
		}

		if y100 == y1100 && se[i].x1 < pt.X {
			// We are going through a vertex.
			if y2100 <= y100 {
				count++
			}
			continue
		}
		if y100 == y2100 && se[i].x2 < pt.X {
			// We are going through a vertex.
			if y1100 <= y100 {
				count++
			}
			continue
		}

		// the segment is verticle and the x is the same; the point is contained.
		if !se[i].isMDefined && pt.X == se[i].x1 {
			return true
		}

		if pt.X > se[i].x2 {
			count++
			continue
		}

		// need to solve for y.
		// y = mx + b
		y = int64((se[i].m*pt.X + se[i].b) * 100)
		if y == y100 {
			return true
		}

		if (se[i].m < 0 && y < y100) ||
			(se[i].m > 0 && y > y100) {
			count++
			continue
		}
	}
	return count%2 != 0

}

type Segment struct {
	bbox   bbox
	label  maths.Label
	events segEvents
}

func (seg Segment) Contains(pt maths.Pt) bool {

	// Check to make sure that the point is within the bounding box.
	if !seg.bbox.Contains(pt) {
		return false
	}
	return seg.events.Contains(pt)
}

func NewSegment(label maths.Label, linestring tegola.LineString) (seg Segment) {

	subpts := linestring.Subpoints()

	seg.label = label
	seg.events = make(segEvents, 0, len(subpts))

	j := len(subpts) - 1
	for i := range subpts {
		l := maths.Line{
			maths.Pt{subpts[j].X(), subpts[j].Y()},
			maths.Pt{subpts[i].X(), subpts[i].Y()},
		}
		seg.bbox.Add(l[:]...)
		seg.events.Add(l)
		j = i
	}
	sort.Sort(seg.events)
	return seg
}

func NewSegmentFromRing(label maths.Label, ring []maths.Pt) (seg Segment) {
	seg.label = label
	seg.events = make(segEvents, 0, len(ring))

	j := len(ring) - 1
	seg.bbox.f = points.BBox(ring)
	seg.bbox.init = true
	for i := range ring {
		l := maths.Line{ring[j], ring[i]}
		seg.events.Add(l)
		j = i
	}
	sort.Sort(seg.events)
	return seg
}
func NewSegmentFromLines(label maths.Label, lines []maths.Line) (seg Segment) {
	seg.label = label
	seg.events = make(segEvents, 0, len(lines))
	for i := range lines {
		seg.bbox.Add(lines[i][:]...)
		seg.events.Add(lines[i])
	}
	sort.Sort(seg.events)
	return seg
}

type M struct {
	s      []Segment
	DoClip bool
	Clip   maths.Rectangle
}

func (hm *M) AppendSegment(seg ...Segment) *M {
	hm.s = append(hm.s, seg...)
	return hm
}

func (hm *M) LabelFor(pt maths.Pt) maths.Label {
	if hm == nil {
		return maths.Outside
	}
	if hm.DoClip {
		if !hm.Clip.Contains(pt) {
			return maths.Outside
		}
	}
	if len(hm.s) == 0 {
		return maths.Outside
	}
	for i := len(hm.s) - 1; i >= 0; i-- {
		if hm.s[i].Contains(pt) {
			return hm.s[i].label
		}
	}
	return maths.Outside
}

func NewFromPolygon(p tegola.Polygon) (hm M) {
	sl := p.Sublines()
	if len(sl) == 0 {
		return hm
	}
	hm.s = make([]Segment, len(sl))
	hm.s[0] = NewSegment(maths.Inside, sl[0])
	for i := range sl[1:] {
		hm.s[i+1] = NewSegment(maths.Outside, sl[i+1])
	}
	return hm
}

func NewFromMultiPolygon(mp tegola.MultiPolygon) (hm M) {
	plgs := mp.Polygons()
	for i := range plgs {
		hm.s = append(hm.s, NewFromPolygon(plgs[i]).s...)
	}
	return hm
}

func NewFromGeometry(g tegola.Geometry) (hm M) {
	switch gg := g.(type) {
	case tegola.Polygon:
		//log.Printf("returning hitmap: hitmap.NewFromPolygon(\n%#v\n)", gg)
		return NewFromPolygon(gg)
	case tegola.MultiPolygon:
		//log.Printf("returning hitmap: hitmap.NewFromMultiPolygon(\n%#v\n)", gg)
		return NewFromMultiPolygon(gg)
	default:
		//log.Println("Returning default hm")
		return hm
	}
}

// NewFromLines creates a new hitmap where the first ring (made up of lines) is considered inside. The others if there are any are considered outside.
func NewFromLines(ln [][]maths.Line) (hm M) {
	hm.s = make([]Segment, len(ln))
	hm.s[0] = NewSegmentFromLines(maths.Inside, ln[0])
	for i := range ln[1:] {
		hm.s[i+1] = NewSegmentFromLines(maths.Outside, ln[i+1])
	}
	return hm
}
