package maths

import (
	"sort"
)

type eventType uint8

const (
	LEFT eventType = iota
	RIGHT
)

type event struct {
	edge     int       // the index number of the edge in the segment list.
	edgeType eventType // Is this the left or right edge.
	ev       *Pt       // event vertex
}

func (e *event) Point() *Pt {
	return e.ev
}

func (e *event) Edge() int {
	return e.edge
}

type Eventer interface {
	Point() *Pt
	Edge() int
}

type XYOrderedEventPtr []event

func (a XYOrderedEventPtr) Len() int           { return len(a) }
func (a XYOrderedEventPtr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a XYOrderedEventPtr) Less(i, j int) bool { return XYOrder(*(a[i].ev), *(a[j].ev)) == -1 }

// Code adapted from http://geomalgorithms.com/a09-_intersect-3.html#simple_Polygon()
func NewEventQueue(segments []Line) []event {

	ne := len(segments) * 2

	eq := make([]event, ne)

	// Initialize event queue with edge segment endpoints
	for i := range segments {
		idx := 2 * i
		eq[idx].edge = i
		eq[idx+1].edge = i
		eq[idx].ev = &(segments[i][0])
		eq[idx+1].ev = &(segments[i][1])
		if XYOrder(segments[i][0], segments[i][1]) < 0 {
			eq[idx].edgeType = LEFT
			eq[idx+1].edgeType = RIGHT
		} else {
			eq[idx].edgeType = RIGHT
			eq[idx+1].edgeType = LEFT
		}
	}
	sort.Sort(XYOrderedEventPtr(eq))
	return eq
}

// DoesIntersect does a quick intersect check using the saddle method.
func DoesIntersect(s1, s2 Line) bool {

	as2 := s2.LeftRightMostAsLine()
	as1 := s1.LeftRightMostAsLine()

	lsign := as1.IsLeft(as2[0]) // s2 left point sign
	rsign := as1.IsLeft(as2[1]) // s2 right point sign
	if lsign*rsign > 0 {        // s2 endpoints have same sign  relative to s1
		return false // => on same side => no intersect is possible
	}

	lsign = as2.IsLeft(as1[0]) // s1 left point sign
	rsign = as2.IsLeft(as1[1]) // s1 right point sign
	if lsign*rsign > 0 {       // s1 endpoints have same sign  relative to s2
		return false // => on same side => no intersect is possible
	}
	// the segments s1 and s2 straddle each other
	return true //=> an intersect exists

}

func FindIntersectsWithEventQueue(polygonCheck bool, eq []event, segments []Line, fn func(srcIdx, destIdx int, ptfn func() Pt) bool) {
	ns := len(segments)
	var val struct{}

	isegmap := make(map[int]struct{})
	for _, ev := range eq {
		_, ok := isegmap[ev.edge]

		if !ok {
			// have not seen this edge, let's add it to our list.
			isegmap[ev.edge] = val
			continue
		}

		// We have reached the end of a segment.
		// This is the left edge.
		delete(isegmap, ev.edge)
		if len(isegmap) == 0 {
			// no segments to test.
			continue
		}
		edge := segments[ev.edge]

		for s := range isegmap {

			src, dest := (s+1)%ns, (ev.edge+1)%ns

			if ev.edge == s {
				continue
			}
			if polygonCheck && (src == ev.edge || dest == s) {
				continue // no non-simple intersect since consecutive
			}

			sedge := segments[s]
			if !DoesIntersect(edge, sedge) {
				continue
			}

			ptfn := func() Pt {
				// Finding the intersect is cpu costly, so wrap it in a function so that if one does not
				// need the intersect the work can be ignored.
				// TODO:gdey — Is this really true? We should profile this to see if this is something that is needed or premature optimaization.
				pt, _ := Intersect(edge, sedge)
				return pt
			}
			src, dest = ev.edge, s
			if src > dest {
				src, dest = dest, src
			}
			if !fn(src, dest, ptfn) {
				return
			}
		}
	}
	return
}

// FindIntersects call the provided function with the indexs of the lines from the segments slice that intersect with each other. If the function returns false, it will stop iteration.
// To find the intersection point call the ptfn that is passed to the call back.
func FindIntersects(segments []Line, fn func(srcIdx, destIdx int, ptfn func() Pt) bool) {
	/*
		ns := len(segments)
		if ns < 3 {
			return
		}
	*/
	eq := NewEventQueue(segments)
	FindIntersectsWithEventQueue(false, eq, segments, fn)
	return
}

// FindPolygonIntersects calls the provided function with the indexes of the lines from the segments slice that intersect with each other. If the function returns false, it will stop iteration.
// To find the intersection point call the ptfn that is passed to the call back.
// The function assumes that the the segments are consecutive.
func FindPolygonIntersects(segments []Line, fn func(srcIdx, destIdx int, ptfn func() Pt) bool) {
	ns := len(segments)
	if ns < 3 {
		return
	}
	eq := NewEventQueue(segments)
	FindIntersectsWithEventQueue(true, eq, segments, fn)
	return
}

//  =================================== LINE methods ================================================= //
func (s1 Line) DoesIntersect(s2 Line) bool {
	return DoesIntersect(s1, s2)
}

// IntersectsLines call fn with line, and intersect point that the line intersects with. from the given set of lines that intersect this line and their intersect points.
func (l Line) IntersectsLines(lines []Line, fn func(idx int) bool) {

	if len(lines) == 0 {
		return
	}
	if len(lines) == 1 {
		if l.DoesIntersect(lines[0]) {
			fn(0)
		}
		return
	}

	// We are going to be using the sweep line method to find intersect points. We are going to modify this method, a bit.
	// We want the line we are trying to find an intersection against as line 0 always. This makes it easy to identify when we don't have to keep looking.
	eq := NewEventQueue(append([]Line{l}, lines...))
	var val struct{}

	isegmap := make(map[int]struct{})

	for _, ev := range eq {

		if _, ok := isegmap[ev.edge]; !ok {
			// have not seen this edge, let's add it to our list.
			isegmap[ev.edge] = val
			continue
		}

		// We have reached the end of a segment.
		// This is the left edge.
		delete(isegmap, ev.edge)
		if len(isegmap) == 0 {
			// no segments to test.
			continue
		}

		// Main edge is always zero
		if ev.edge == 0 {
			// Look through the edges in our map, comparing it to the main line, as we came to end of the main line.
			for s := range isegmap {
				if !l.DoesIntersect(lines[s-1]) {
					continue
				}
				// We found an intersect, let someone know.
				if !fn(s - 1) {
					return
				}
			}
			// We are done with the main edge, don't care if the other intersect.
			return
		}

		// Is the main line in the edge map, if not we don't care if there are any intersects with this line.
		if _, ok := isegmap[0]; !ok {
			continue
		}

		// Let's see if there is an intersect with the main edge.
		if !l.DoesIntersect(lines[ev.edge-1]) {
			// Nope, let's move on.
			continue
		}
		// Yes, let some know.
		if !fn(ev.edge - 1) {
			// They want us to stop.
			return
		}
		continue
	}
}

func (l Line) XYOrderedPtsIdx() (left, right int) {
	if XYOrder(l[0], l[1]) == -1 {
		return 0, 1
	}
	return 1, 0
}
