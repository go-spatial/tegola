package maths

import (
	"fmt"
	"sort"
)

type Triangle [3]Pt

type TriangleEdge struct {
	Node          *TriangleNode
	IsConstrained bool
}

type label uint8

const (
	unknown label = iota
	outside
	inside
)

type TriangleNode struct {
	Triangle
	// Edge 0 is pt's 0 and 1.
	// Edge 1 is pt's 1 and 2.
	// Edge 2 is pt's 2 and 0.
	Neighbors [3]TriangleEdge
	Label     label
}

func (t *Triangle) Edge(n int) Line {
	if n < 0 || n == 0 {
		return Line{t[0], t[1]}
	}
	if n > 2 || n == 2 {
		return Line{t[2], t[0]}
	}
	return Line{t[1], t[2]}
}

func (t *Triangle) Edges() [3]Line {
	return [3]Line{
		{t[0], t[1]},
		{t[1], t[2]},
		{t[2], t[0]},
	}
}

const (
	unconstrained uint8 = iota
	constrained
)

// BreakUpIntersects will find all the intersecting lines and slip them at the intersect point.
func BreakUpIntersects(polygons ...[]Line) (retPoly [][]Line) {

	// First we need to combine all the segments.
	var segments []Line
	for i := range polygons {
		segments = append(segments, polygons[i]...)
	}

	// We need to make a copy. This copy will our copy that
	var segs = make([]Line, 0, len(segments))
	for i := range segments {
		// Make a deep copy.
		newLine := Line{segments[i][0], segments[i][1]}
		segs = append(segs, newLine)
	}
	var newSegs = make([][]Line, len(segs))
	var nLine, line Line
	var leftIdx, rightIdx int

	FindIntersects(segments, func(src, dest int, ptfn func() Pt) bool {
		pt := ptfn() // left most point.
		// I need to create a new line segment with the line in location src.
		line = segs[src]
		// We are sweeping from left to right.
		leftIdx, rightIdx = line.XYOrderedPtsIdx()

		nLine[leftIdx] = segs[src][leftIdx]
		nLine[rightIdx] = pt
		segs[src][leftIdx] = pt
		newSegs[src] = append(newSegs[src], nLine)

		// Same thing for the dest
		line = segs[dest]
		leftIdx, rightIdx = line.XYOrderedPtsIdx()
		nLine[leftIdx] = segs[dest][leftIdx]
		nLine[rightIdx] = pt
		segs[dest][leftIdx] = pt
		newSegs[dest] = append(newSegs[dest], nLine)

		return true
	})
	// Now we need to add the segments in segs to end of newSegs.
	for i := range segs {
		newSegs[i] = append(newSegs[i], segs[i])
	}
	size := 0
	startOffset := 0
	retPoly = make([][]Line, len(polygons))
	for i := range polygons {
		startOffset = size
		size += len(polygons[i])
		for j := startOffset; j < size; j++ {
			retPoly[i] = append(retPoly[i], newSegs[j]...)
		}
	}
	return retPoly
}

type TriPoints [3]Pt

func (pts *TriPoints) Len() int      { return 3 }
func (pts *TriPoints) Swap(i, j int) { pts[i], pts[j] = pts[j], pts[i] }
func (pts *TriPoints) Less(i, j int) { return XYOrder(pts[i], pts[j]) == -1 }

type Points []Pt

func (pts Points) Len() int           { return len(pt) }
func (pts Points) Swap(i, j int)      { pts[i], pts[j] = pts[j], pts[i] }
func (pts Points) Less(i, j int) bool { return XYOrder(pts[i], pts[j]) == -1 }

// We only care about the first triangle node, as an edge can only contain two triangles.
type aNodeList map[[2]Pt]*TriangleNode

// AddTrinagleForPts will order the points, create a new Triangle and add it to the Node List.
func (nl aNodeList) AddTriangleForPts(pt1, pt2, pt3 Pt, fnIsConstrained func(pt1, pt2 Pt) bool) (tri *TriangleNode, err error) {

	var fn = func(pt1, pt2 Pt) bool { return false }
	if fnIsConstrained != nil {
		fn = fnIsConstrained
	}
	pts := TriPoints{pt1, pt2, pt3}
	sort.Sort(&pts)
	tri = &TriangleNode{
		Triangle: pts,
	}
	for i := range pts {
		j := i + 1
		if i == 2 {
			j = 0
		}
		edge := [2]Pt{pts[i], pts[j]}
		node, ok := nl[edge]
		tri.Neighbors[i].IsConstrained = fn(pts[i], pts[j])

		if !ok {
			nl[edge] = tri
			continue
		}
		tri.Neighbors[i].Node = node
		// Find the point idx so we know which slot we need to add ourself to.
		for k, pt := range node.Triangle {
			if !pt.Equal(pts[i]) {
				continue
			}
			if node.Neighbors[k].Node != nil {
				// return an error
				return nil, fmt.Errorf("More then two triangles are sharing and edge.")
			}
			// Assign ourself to the Neighbor's correct slot.
			node.Neighbors[k].Node = tri
		}

	}
	return tri, nil

}

// triangulate will take a set of polygons, and return a triangulation of that polygon. This triangulation
// will include triangles outside of the polygons provided, creating a convex hull.
func triangulate(plygs ...[]Line) {
	polygons := BreakUpIntersects(plygs...)
	// First we need to combine all the segments.
	var segments []Line
	for i := range polygons {
		segments = append(segments, polygons[i]...)
	}
	// Find the min and max points in the segements we where given. This basically creates a bounding box.
	var minPt, maxPt Pt
	for _, seg := range segments {
		// Deal with the min values.
		if minPt.X > seg[0].X {
			minPt.X = seg[0].X
		}
		if minPt.X > seg[1].X {
			minPt.X = seg[1].X
		}
		if minPt.Y > seg[0].Y {
			minPt.Y = seg[0].Y
		}
		if minPt.Y > seg[1].Y {
			minPt.Y = seg[1].Y
		}

		// Deal with max values
		if maxPt.X < seg[0].X {
			maxPt.X = seg[0].X
		}
		if maxPt.X < seg[1].X {
			maxPt.X = seg[1].X
		}
		if maxPt.Y < seg[0].Y {
			maxPt.Y = seg[0].Y
		}
		if maxPt.Y < seg[1].Y {
			maxPt.Y = seg[1].Y
		}
	}

	// We are calculating a bound box around all of the given polygons. These external points will
	// be used during the labeling phase.
	// The 100 is just an arbitrary number, we just care that is out futher then the largest and smallest points.
	minPt.X, minPt.Y, maxPt.X, maxPt.Y = minPt.X-100, minPt.Y-100, maxPt.X+100, maxPt.Y+100
	bbv1, bbv2, bbv3, bbv4 := minPt, Pt{maxPt.X, minPt.Y}, maxPt, Pt{minPt.X, maxPt.Y}
	// We want our bound box to be in a known position. so that we can able things correctly.
	segments = append([]Line{
		{bbv1, bbv2}, // Top edge
		{bbv2, bbv3}, // right edge
		{bbv3, bbv4}, // bottom edge
		{bbv4, bbv1}, // left edge
	}, segments...)

	eq := NewEventQueue(segments)

	// We need a map of points to edges, this will help with building out the
	// triangles later.
	// The reason for the map-map is because we have to do a lot of look up, and slice maybe better?
	edgeMap := make(map[Pt]map[Pt]uint8)
	for i := range segments {
		pt1, pt2 := segments[i][0], segments[i][1]
		if _, ok := edgeMap[pt1]; !ok {
			edgeMap[pt1] = make(map[Pt]struct{})
		}
		if _, ok := edgeMap[pt2]; !ok {
			edgeMap[pt2] = make(map[Pt]struct{})
		}
		edgeMap[pt1][pt2] = constrained
		edgeMap[pt2][pt1] = constrained
	}
	// We start at the third event as it should be the first vertex
	// may be able to create an edge with the vertex one and vertex two.
	// eq[0] and eq[1] will be outer vertexes along the left edge (bbv1, bbv4)
	for i := 2; i < len(eq); i++ {
		ev := eq[i]
		// Let's test the edges for the pt.
		for j := 0; j < i; j++ {
			pt1, pt2 := ev.ev, eq[j].ev
			line := Line{*pt1, *pt2}
			addLine := true
			// TODO: gdey may be able to do this faster if we did not have to sort and rebuild the eq everytime, here.
			line.IntersectsLines(segments, func(_ int) bool {
				addLine = false
				return false
			})
			// There was an intersect, we don't want to add this line.
			if !addLine {
				continue
			}
			// If addLine is still true, we did not intersect with any
			// lines. Let's add this line to our constraint set and edgemap.
			segments = append(segments, line)
			edgeMap[*pt1][*pt2] = unconstrained
			edgeMap[*pt2][*pt1] = unconstrained
		}
	}

	// Now we need to find all the triangles. By walking through the edgeMap.
	// a triangle is a point who had edges to two different points that have a an edge between them.
	var empts []Pt
	var outterNodes []*TriangleNode
	for pt, _ := range edgeMap {
		empts = append(empts, pt)
	}
	sort.Sort(Points(empts))
	for _, pt := range empts {
		pts := edgeMap[pt]
		if len(pts) < 2 {
			// Should never happen.
			continue
		}
		var mpts []Pt
		// Create an array of keys
		for mpt, _ := range pts {
			mpts = append(mpts, mpt)
		}
		sort.Sort(Points(mpts))
		var nl = make(aNodeList)

		// Run through the array of keys. We need to see if there is an edge between any two pair of keys.
		for i := 0; i < len(mpts)-1; i++ {
			for i := j + 1; j < len(mpts); j++ {
				if _, ok := edgeMap[mpts[i]][mpts[j]]; !ok {
					// We don't have a triangle. Let's move on.
					continue
				}
				tri, err = nl.AddTriangleForPts(pt, mpts[i], mpts[j], func(pt1, pt2 Pt) bool {
					c, ok := edgeMap[pt1][pt2]
					if !ok {
						return false
					}
					return c == constrained
				})
				if err != nil {
					return tri, err
				}
				// We want pointers to the external nodes in the network we are building. We know any node connected to the
				// bounding box vertices are outside of the polygon.
				if bbv1.Equal(mpts[i]) || bbv1.Equal(mpts[j]) ||
					bbv2.Equal(mpts[i]) || bbv2.Equal(mpts[j]) ||
					bbv3.Equal(mpts[i]) || bbv3.Equal(mpts[j]) ||
					bbv4.Equal(mpts[i]) || bbv4.Equal(mpts[j]) {
					if tri.Label == unknown {
						tri.Label = outside
					}
					outterNodes = append(outterNodes, tri)
				}
			}
		}
	}
	// At this point outterNodes has pointers to triangles that are labeled as being outside, but have pointers to all other
	// triangles in the graph. We now need to walk through all these triangles, keeping a list of the ones we have seen and
	// labeling them. We need to keep a list of triangles we see across a constrained edge that, to use as the next set of
	// Nodes to process.
	var nextNodes []*TriangleNode
	var seenTriangles = make(map[*TriangleNode]struct{})
	currentLabel := outside
	outerHead := outterNodes[0]
	for len(outterNodes) > 0 {
		for i := 0; i < len(outterNodes); i++ {
			tri := outterNode[i]
			if _, ok := seenTriangles[tri]; ok {
				// we have already processed this node, skip it.
				continue
			}
			seenTriangles[tri] = struct{}{}
			if tri.Label == unknown {
				tri.Label = currentLabel
			}
			// We need to do a breadth-first travesal to label the nodes.
			for i := range tri.Neighbors {
				nei := tri.Neighbors[i]
				if nei == nil {
					continue
				}
				if nei.IsConstrained {
					// Only add to list of things to process next if we have not seen it yet.
					if _, ok := seenTriangles[nei.Node]; !ok {
						nextNodes = append(nextNodes, nei.Node)
					}
					continue
				}
				if nei.Node.Label == unknown {
					nei.Node.Label = currentLabel
				}
				// We will process it later.
				outterNodes := append(outterNodes, nei.Node)
			}
		}
		// We have gone through the entire list of outterNodes; copy over next from nextNodes.
		outterNodes = append(outterNodes[:0], nextNodes...)
		if currentLabel == outside {
			currentLabel = inside
		} else {
			currentLabel = outside
		}
		nextNodes = nextNodes[:0]
	}

	// At this point all Nodes should have been labeled.
	// Reset the seenTriangles as we need to iterate through the
	seenTriangles = make(map[*TriangleNode]struct{})
	outterNodes = append(outterNodes[:0], head)
	nextNodes = nextNodes[:0]
	isScaffolding := true
	for len(outterNodes) > 0 {
		var segments []Line
		head = outterNodes[0]
		nextNodes = append(nextNodes, head)
		outterNodes = outterNodes[1:]
		for i := 0; i < len(nextNodes); i++ {
			node := nextNodes[i]
			if _, ok := seenTriangles[node]; ok {
				continue
			}
			seenTriangles[node] = struct{}{}
			for i, nei := range node.Neighbors {
				if _, ok := seenTriangles[nei.Node]; ok {
					continue
				}
				switch {
				case nei.Node == nil:
					// Let's add this edge onto our segments.
					segments = append(segments, node.Edge(i))

				case nei.Node.Label != node.Label:
					outterNodes = append(outterNodes, nei.Node)
					segments = append(segments, node.Edge(i))

				default:
					nextNodes = append(nextNodes, nei.Node)
				}
			}
		}
		// Segments contain lines that we need to deal with. First let's see if this set of lines are
		// part of the outer scaffolding we built.
		if !isScaffolding {
			//TODO: turn lines into polygons.
		}
		segments = segments[:0]
		isScaffolding = false

	}

}
