// +build fiBtree,!fiArray,!fiLinkedList

package edgemap

import (
	"errors"
	"fmt"

	"github.com/terranodo/tegola/maths"
)

var intersectname = "fiBTree"

type eventNode struct {
	pt    maths.Pt
	edges []int
	left  *eventNode
	right *eventNode
}

func (n *eventNode) Insert(pt *maths.Pt, edge int) error {
	if n == nil {
		return errors.New("Cannot insert a value into a nil tree.")
	}
	if pt.X == n.pt.X && pt.Y == n.pt.Y {
		n.edges = append(n.edges, edge)
		return nil
	}
	if pt.X < n.pt.X || (pt.X == n.pt.X && pt.Y < n.pt.Y) {
		// point is less then node.
		if n.left == nil {
			n.left = &eventNode{pt: *pt, edges: []int{edge}}
			return nil
		}
		return n.left.Insert(pt, edge)
	}
	if n.right == nil {
		n.right = &eventNode{pt: *pt, edges: []int{edge}}
		return nil
	}
	return n.right.Insert(pt, edge)
}

type eventTree struct {
	root *eventNode
	len  int
}

func (t *eventTree) Insert(pt *maths.Pt, edge int) (err error) {
	if t.root == nil {
		t.root = &eventNode{pt: *pt, edges: []int{edge}}
		t.len = 1
		return nil
	}
	if err = t.root.Insert(pt, edge); err == nil {
		t.len = t.len + 1
	}
	return err
}

func findIntersects(segments []maths.Line, skiplines []bool) {
	fmt.Println("Intersect Btree")

	var offset = len(skiplines)
	var segLen = len(segments)
	var isegmap = make([]bool, segLen)
	var i, oldline, idx, seenEdgeCount, sl int

	var eqTree eventTree
	var n *eventNode
	var enableids, edges []int

	for i = range segments {
		eqTree.Insert(&(segments[i][0]), i)
		eqTree.Insert(&(segments[i][1]), i)
	}

	n = eqTree.root
	var nodes = make([]*eventNode, 0, eqTree.len/2)
	for {
		for n.left != nil {
			// need to process later.
			nodes = append(nodes, n)
			n = n.left
		}
		// do stuff with n.
		for _, idx = range n.edges {
			if !isegmap[idx] {
				enableids = append(enableids, idx)
				continue
			}
			isegmap[idx] = false
			edges = append(edges, idx)
		}
		seenEdgeCount = seenEdgeCount - len(edges)
		if seenEdgeCount <= 0 {
			edges = edges[:0]
		}

		// We need to loop through our edges to find the ones that are intersecting.
		for _, idx = range edges {
			if idx >= offset {
				// skip all segments greater then offset, as constraints should not intersect.
				sl = offset
			} else {
				sl = segLen
			}
			for oldline = 0; oldline < sl; oldline++ {
				for ; oldline < sl && !isegmap[oldline]; oldline++ {
					// skip forward.
				}
				if oldline >= sl {
					break
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

		seenEdgeCount = 0 + len(enableids)
		for _, idx = range enableids {
			isegmap[idx] = true
		}
		edges = edges[:0]
		enableids = enableids[:0]

		// Do we need to go again?
		if n.right != nil {
			n = n.right
			continue
		}
		if len(nodes) != 0 {
			n = nodes[len(nodes)-1]
			nodes = nodes[:len(nodes)-1]
			continue
		}
		break
	}

	return
}
