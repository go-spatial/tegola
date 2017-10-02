// +build !fiMapArray,!fiLinkedList,!fiBtree

package edgemap

import (
	"github.com/terranodo/tegola/maths"
)

var intersectname = "fiArray"

func findIntersects(segments []maths.Line, skiplines []bool) {
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
				skiplines[idx] = true
				continue
			}
			// If idx >= offset then s has to be < offset. because of sl.
			skiplines[s] = true
		}
	}

	return
}
