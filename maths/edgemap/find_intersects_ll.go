// +build fiLinkedList,!fiArray,!fiBtree

package edgemap

// Tests have proven this to not be as fast as just having an array of booleans.

import (
	"github.com/terranodo/tegola/maths"
)

var intersectname = "fiLinkedList"

type edgeNode struct {
	edge int
	next *edgeNode
}

type edgeList struct {
	head *edgeNode
	tail *edgeNode
	len  int
}

func (el *edgeList) Find(edge int) bool {
	var en = el.head
	for en != nil && en.edge != edge {
		en = en.next
	}
	return en != nil
}
func (el *edgeList) NotInList(edge int) bool {
	var en = el.head
	for en != nil && en.edge != edge {
		en = en.next
	}
	return en == nil
}

func (el *edgeList) Remove(edge int) {
	if el.head == nil {
		return
	}
	var indirect = &el.head
	var en = el.head
	for en != nil && en.edge != edge {
		indirect, en = &en.next, en.next
	}
	if en != nil {
		(*indirect) = en.next
		el.len = el.len - 1
	}

	if edge == el.tail.edge {
		el.tail = el.head
		for ; el.tail != nil && el.tail.next != nil; el.tail = el.tail.next {
		}
	}
}

func (el *edgeList) Add(edge int) bool {
	if el.Find(edge) {
		return false
	}
	if el.head == nil {
		el.head = &edgeNode{edge: edge}
		el.tail = el.head
	}
	el.tail.next = &edgeNode{edge: edge}
	el.len = el.len + 1
	return true
}

func findIntersects(segments []maths.Line, skiplines []bool) {
	var offset = len(skiplines)
	var segLen = len(segments)
	var isegmap edgeList

	var eq = make([]event, segLen*2)
	var i, oldline, idx, sl int

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
		if isegmap.Add(idx) {
			continue
		}
		isegmap.Remove(idx)
		if isegmap.len == 0 {
			continue
		}

		if idx >= offset {
			// skip all segments greater then offset, as constraints should not intersect.
			sl = offset
		} else {
			sl = segLen
		}

		for h := isegmap.head; h != nil; h = h.next {
			oldline = h.edge
			if oldline >= sl {
				continue
			}
			if idx < offset && skiplines[idx] {
				continue
			}
			if oldline < offset && skiplines[oldline] {
				continue
			}
			if segments[idx][0].X == segments[oldline][0].X && segments[idx][0].Y == segments[oldline][0].Y {
				continue
			}
			if segments[idx][0].X == segments[oldline][1].X && segments[idx][0].Y == segments[oldline][1].Y {
				continue
			}
			if segments[idx][1].X == segments[oldline][0].X && segments[idx][1].Y == segments[oldline][0].Y {
				continue
			}
			if segments[idx][1].X == segments[oldline][1].X && segments[idx][1].Y == segments[oldline][1].Y {
				continue
			}
			if doesNotIntersect(segments[idx][0].X, segments[idx][0].Y, segments[idx][1].Y, segments[idx][1].Y, segments[oldline][0].X, segments[oldline][0].Y, segments[oldline][1].X, segments[oldline][1].Y) {
				continue
			}
			// If the current line is a possible edge, let's mark that one to be skipped, and keep
			// the lines that extend futher to the lower right
			if oldline < offset {
				skiplines[oldline] = true
			} else {
				skiplines[idx] = true
			}
		}
	}
	return
}
