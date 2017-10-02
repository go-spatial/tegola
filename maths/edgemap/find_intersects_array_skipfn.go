// +build !fiLinkedList,!fiBtree

package edgemap

import "github.com/terranodo/tegola/maths"

// DoesIntersect does a quick intersect check using the saddle method.
func findinter_doesNotIntersect(s1x0, s1y0, s1x1, s1y1, s2x0, s2y0, s2x1, s2y1 float64) bool {

	var swap float64

	// Put line 1 points in order.
	if s1x0 > s1x1 {
		swap = s1x0
		s1x0 = s1x1
		s1x1 = swap

		swap = s1y0
		s1y0 = s1y1
		s1y1 = swap
	} else {
		if s1x0 == s1x1 && s1y0 > s1y1 {
			swap = s1x0
			s1x0 = s1x1
			s1x1 = swap

			swap = s1y0
			s1y0 = s1y1
			s1y1 = swap
		}
	}
	// Put line 2 points in order.
	if s2x0 > s2x1 {
		swap = s2x0
		s2x0 = s2x1
		s2x1 = swap

		swap = s2y0
		s2y0 = s2y1
		s2y1 = swap
	} else {
		if s2x0 == s2x1 && s2y0 > s2y1 {
			swap = s2x0
			s2x0 = s2x1
			s2x1 = swap

			swap = s2y0
			s2y0 = s2y1
			s2y1 = swap
		}
	}

	if ((((s1x1 - s1x0) * (s2y0 - s1y0)) - ((s1y1 - s1y0) * (s2x0 - s1x0))) * (((s1x1 - s1x0) * (s2y1 - s1y0)) - ((s1y1 - s1y0) * (s2x1 - s1x0)))) > 0 {
		return true
	}
	if ((((s2x1 - s2x0) * (s1y0 - s2y0)) - ((s2y1 - s2y0) * (s1x0 - s2x0))) * (((s2x1 - s2x0) * (s1y1 - s2y0)) - ((s2y1 - s2y0) * (s1x1 - s2x0)))) > 0 {
		return true
	}

	return false

}

func findIntersectsSkip(segments []maths.Line, skiplines []bool) {
	var offset = len(skiplines)
	var segLen = len(segments)

	var isegmap = make([]bool, segLen)
	seenEdgeCount := 0

	var s, i, idx, sl int

	var eq = make([]event, segLen*2)
	for i = range segments {
		eq[idx].edge = i
		eq[idx+1].edge = i
		eq[idx].pt = &(segments[i][0])
		eq[idx+1].pt = &(segments[i][1])
		idx = idx + 2
	}
	byXY(eq).Sort()

	for i = range eq {
		idx = eq[i].edge

		if !isegmap[idx] {
			// have not seen this edge, let's add it to our list.
			isegmap[idx] = true
			seenEdgeCount = seenEdgeCount + 1
			continue
		}

		// We have reached the end of a segment.
		// This is the left edge.
		isegmap[idx] = false
		seenEdgeCount = seenEdgeCount - 1
		if seenEdgeCount <= 0 {
			seenEdgeCount = 0
			// no segments to test.
			continue
		}
		if idx >= offset {
			sl = offset
		} else {
			sl = segLen
		}

		for s = 0; s < sl; s++ {
			for ; s < sl && !isegmap[s]; s++ {
				// skip forward.
			}
			if s >= sl {
				break
			}
			if idx < offset && skiplines[idx] {
				continue
			}
			if s < offset && skiplines[s] {
				continue
			}
			if segments[idx][0].X == segments[s][0].X && segments[idx][0].Y == segments[s][0].Y {
				continue
			}
			if segments[idx][0].X == segments[s][1].X && segments[idx][0].Y == segments[s][1].Y {
				continue
			}
			if segments[idx][1].X == segments[s][0].X && segments[idx][1].Y == segments[s][0].Y {
				continue
			}
			if segments[idx][1].X == segments[s][1].X && segments[idx][1].Y == segments[s][1].Y {
				continue
			}
			if doesNotIntersect(segments[idx][0].X, segments[idx][0].Y, segments[idx][1].X, segments[idx][1].Y, segments[s][0].X, segments[s][0].Y, segments[s][1].X, segments[s][1].Y) {
				continue
			}
			if idx < offset {
				if s >= offset {
					skiplines[idx] = true
				}
				continue
			}
			if s < offset {
				if idx >= offset {
					skiplines[s] = true
				}
				continue
			}

			//fn(idx, s)
		}
	}
	return
}
func (em *EM) Triangulate1() {
	//defer log.Println("Done with Triangulate")
	keys := em.Keys
	lnkeys := len(keys) - 1
	//log.Println("Starting to Triangulate. Keys", len(keys))
	// We want to run through all the keys generating possible edges, and then
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
		lines := append([]maths.Line{}, possibleEdges...)
		offset := len(lines)
		lines = append(lines, em.Segments...)
		skiplines := make([]bool, offset)

		findIntersects(lines, skiplines)
		// Add the remaining possible Edges to the edgeMap.
		for i := range possibleEdges {
			if skiplines[i] {
				continue
			}
			// Don't need to add the keys as they are already in the edgeMap, we are just adding additional edges
			// between points.
			em.addLine(false, true, false, possibleEdges[i])
		}
	}
}

/*
func findIntersectsSkip(segments []maths.Line, skiplines []bool, skipfn func(srcIdx, destIdx int) bool, fn func(srcIdx, destIdx int)) {
	ns := len(segments)
	isegmap := make([]bool, ns)
	seenEdgeCount := 0
	var edgeidx, s, i, idx int
	var sv bool

	var eq = make([]event, ns*2)
	for i = range segments {
		eq[idx].edge = i
		eq[idx+1].edge = i
		eq[idx].pt = &(segments[i][0])
		eq[idx+1].pt = &(segments[i][1])
		idx = idx + 2
	}
	byXY(eq).Sort()

	for i := range eq {
		edgeidx = eq[i].edge

		if !isegmap[edgeidx] {
			// have not seen this edge, let's add it to our list.
			isegmap[edgeidx] = true
			seenEdgeCount++
			continue
		}

		// We have reached the end of a segment.
		// This is the left edge.
		isegmap[edgeidx] = false
		seenEdgeCount = seenEdgeCount - 1
		if seenEdgeCount <= 0 {
			seenEdgeCount = 0
			// no segments to test.
			continue
		}

		for s, sv = range isegmap {
			if !sv {
				continue
			}
			if skipfn(edgeidx, s) {
				continue
			}
			if segments[edgeidx][0].X == segments[s][0].X && segments[edgeidx][0].Y == segments[s][0].Y {
				continue
			}
			if segments[edgeidx][0].X == segments[s][1].X && segments[edgeidx][0].Y == segments[s][1].Y {
				continue
			}
			if segments[edgeidx][1].X == segments[s][0].X && segments[edgeidx][1].Y == segments[s][0].Y {
				continue
			}
			if segments[edgeidx][1].X == segments[s][1].X && segments[edgeidx][1].Y == segments[s][1].Y {
				continue
			}
			if doesNotIntersect(segments[edgeidx][0].X, segments[edgeidx][0].Y, segments[edgeidx][1].X, segments[edgeidx][1].Y, segments[s][0].X, segments[s][0].Y, segments[s][1].X, segments[s][1].Y) {
				continue
			}

			fn(edgeidx, s)
		}
	}
	return
}
*/
