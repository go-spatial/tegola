package edgemap

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

func trace(msg string) func() {
	tracer := time.Now()
	log.Println(msg, "started at:", tracer)
	return func() {
		etracer := time.Now()
		log.Println(msg, "ElapsedTime in seconds:", etracer.Sub(tracer))
	}
}

const adjustBBoxBy = 10

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
}

type EM struct {
	Keys         []maths.Pt
	Map          map[maths.Pt]map[maths.Pt]bool
	Segments     []maths.Line
	BBox         [4]maths.Pt
	destructured []maths.Line
}

func (em *EM) SubKeys(pt maths.Pt) (skeys []maths.Pt, ok bool) {
	sem, ok := em.Map[pt]
	if !ok {
		return skeys, false
	}
	for k := range sem {
		if k.IsEqual(pt) {
			continue
		}
		skeys = append(skeys, k)
	}
	sort.Sort(points.ByXY(skeys))
	return skeys, ok
}

func (em *EM) TrianglesForEdge(pt1, pt2 maths.Pt) (*maths.Triangle, *maths.Triangle, error) {

	apts, ok := em.SubKeys(pt1)
	if !ok {
		log.Println("Error 1")
		return nil, nil, fmt.Errorf("Point one is not connected to any other points. Invalid edge? (%v  %v)", pt1, pt2)
	}
	bpts, ok := em.SubKeys(pt2)
	if !ok {
		log.Println("Error 2")
		return nil, nil, fmt.Errorf("Point two is not connected to any other points. Invalid edge? (%v  %v)", pt1, pt2)
	}

	// Check to make sure pt1 and pt2 are connected.
	if _, ok := em.Map[pt1][pt2]; !ok {
		log.Println("Error 3")
		return nil, nil, fmt.Errorf("Point one and Point do not form an edge. Invalid edge? (%v  %v)", pt1, pt2)
	}

	// Now we need to look at the both subpts and only keep the points that are common to both lists.
	var larea, rarea float64
	var triangles [2]*maths.Triangle
NextApts:
	for i := range apts {
	NextBpts:
		for j := range bpts {
			if apts[i].IsEqual(bpts[j]) {
				tri := maths.NewTriangle(pt1, pt2, apts[i])
				area := maths.AreaOfTriangle(pt1, pt2, apts[i])
				switch {
				case area > 0 && (rarea == 0 || rarea > area):
					rarea = area
					triangles[1] = &tri
				case area < 0 && (larea == 0 || larea < area):
					larea = area
					triangles[0] = &tri
				case area == 0:
					// Skip lines with zero area.
					continue NextBpts
				}
				continue NextApts
			}
		}
	}
	return triangles[0], triangles[1], nil
}

func New(lines []maths.Line) (em *EM) {
	em = new(EM)

	em.destructured = lines
	em.Map = make(map[maths.Pt]map[maths.Pt]bool)
	em.Segments = make([]maths.Line, 0, len(lines))

	// First we need to combine all the segments.
	var minPt, maxPt, midPt maths.Pt
	for i := range lines {
		seg := lines[i]
		pt1, pt2 := seg[0], seg[1]
		if pt1.IsEqual(pt2) {
			continue // skip point lines
		}
		if _, ok := em.Map[pt1]; !ok {
			em.Map[pt1] = make(map[maths.Pt]bool)
		}
		if _, ok := em.Map[pt2]; !ok {
			em.Map[pt2] = make(map[maths.Pt]bool)
		}
		if _, ok := em.Map[pt1][pt2]; ok {
			// skip this point as we have already dealt with this point set.
			continue
		}

		em.Segments = append(em.Segments, seg)

		em.Map[pt1][pt2] = true
		em.Map[pt2][pt1] = true

		// Find the min and max points in the segements we where given. This basically creates a bounding box.
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
	// Build out the keys:
	for k := range em.Map {
		em.Keys = append(em.Keys, k)
	}
	// We are calculating a bound box around all of the given polygons. These external points will
	// be used during the labeling phase.
	// The adjustBBoxBy is just an arbitrary number, we just care that is out futher then the largest and smallest points.
	minPt.X, minPt.Y, maxPt.X, maxPt.Y = minPt.X-adjustBBoxBy, minPt.Y-adjustBBoxBy, maxPt.X+adjustBBoxBy, maxPt.Y+adjustBBoxBy
	midPt.X, midPt.Y = float64(int64((maxPt.X-minPt.X)/2)), float64(int64((maxPt.Y-minPt.Y)/2))

	bbv1, bbv2, bbv3, bbv4 := minPt, maths.Pt{maxPt.X, minPt.Y}, maxPt, maths.Pt{minPt.X, maxPt.Y}

	// We want our bound box to be in a known position. so that we can able things correctly.
	em.Segments = append([]maths.Line{
		{bbv1, bbv2},  // Top edge
		{bbv2, bbv3},  // right edge
		{bbv3, bbv4},  // bottom edge
		{bbv4, bbv1},  // left edge
		{bbv1, midPt}, // Top edge
		{bbv2, midPt}, // right edge
		{bbv3, midPt}, // bottom edge
		{bbv4, midPt}, // left edge

	}, em.Segments...)
	em.BBox = [4]maths.Pt{bbv1, bbv2, bbv3, bbv4}
	em.addLine(false, false, true, em.Segments[0:8]...)

	keys := em.Keys
	sort.Sort(points.ByXY(keys))
	em.Keys = []maths.Pt{keys[0]}
	lk := keys[0]
	for _, k := range keys[1:] {
		if lk.IsEqual(k) {
			continue
		}
		em.Keys = append(em.Keys, k)
		lk = k
	}
	return em
}

func (em *EM) AddLine(constrained bool, addSegments bool, addKeys bool, lines ...maths.Line) {
	em.addLine(constrained, addSegments, addKeys, lines...)
}

func (em *EM) addLine(constrained bool, addSegments bool, addKeys bool, lines ...maths.Line) {
	if em == nil {
		return
	}
	if em.Map == nil {
		em.Map = make(map[maths.Pt]map[maths.Pt]bool)
	}
	for _, l := range lines {
		pt1, pt2 := l[0], l[1]
		if _, ok := em.Map[pt1]; !ok {
			em.Map[pt1] = make(map[maths.Pt]bool)
		}
		if _, ok := em.Map[l[1]]; !ok {
			em.Map[pt2] = make(map[maths.Pt]bool)
		}
		if con, ok := em.Map[pt1][pt2]; !ok || !con {
			em.Map[pt1][pt2] = constrained
		}
		if con, ok := em.Map[pt2][pt1]; !ok || !con {
			em.Map[pt2][pt1] = constrained
		}
		if addKeys {
			em.Keys = append(em.Keys, pt1, pt2)
		}
		if addSegments {
			em.Segments = append(em.Segments, l)
		}
	}
}

func (em *EM) Dump() {
	/*
		log.Println("Edge Map:")
		log.Printf("generateEdgeMap(%#v)", em.destructured)
		if em == nil {
			log.Println("nil")
		}
		log.Println("\tKeys:", em.Keys)
		log.Println("\tMap:")
		var keys []Pt
		for k := range em.Map {
			keys = append(keys, k)
		}
		sort.Sort(ByXY(keys))
		for _, k := range keys {
			log.Println("\t\t", k, ":\t", len(em.Map[k]), em.Map[k])
		}
		log.Println("\tSegments:")
		for _, seg := range em.Segments {
			log.Println("\t\t", seg)
		}
		log.Printf("\tBBox:%v", em.BBox)
	*/
}

/*
func (em *EM) Triangulate1() {

	//defer log.Println("Done with Triangulate")
	keys := em.Keys

	//log.Println("Starting to Triangulate. Keys", len(keys))
	// We want to run through all the keys generating possible edges, and then
	// collecting the ones that don't intersect with the edges in the map already.
	var lines = make([]maths.Line, 0, 2*len(keys))
	stime := time.Now()
	for i := 0; i < len(keys)-1; i++ {
		lookup := em.Map[keys[i]]
		//log.Println("Looking at i", i, "Lookup", lookup)
		for j := i + 1; j < len(keys); j++ {
			if _, ok := lookup[keys[j]]; ok {
				// Already have an edge with this point
				continue
			}
			l := maths.Line{keys[i], keys[j]}
			lines = append(lines, l)
		}
	}
	etime := time.Now()
	log.Println("Finding all lines took: ", etime.Sub(stime))

	// Now we need to do a line sweep to see which of the possible edges we want to keep.
	offset := len(lines)
	lines = append(lines, em.Segments...)
	// Assume we are going to keep all the edges we generated.
	//skiplines := make([]bool, len(lines))
	skiplines := make(map[int]bool, offset)

	stime = time.Now()
	eq := NewEventQueue(lines)
	etime = time.Now()
	log.Println("building event queue took: ", etime.Sub(stime))
	stime = etime
	FindAllIntersectsWithEventQueueWithoutIntersectNotPolygon(eq, lines,
		func(src, dest int) bool { return skiplines[src] || skiplines[dest] },
		func(src, dest int) {

			if dest < offset {
				skiplines[dest] = true
				return
			}
			if src < offset {
				skiplines[src] = true
				return
			}
		})
	etime = time.Now()
	log.Println("Find Intersects took: ", etime.Sub(stime))
	stime = etime
	// Add the remaining possible Edges to the edgeMap.
	for i := range lines {
		if _, ok := skiplines[i]; ok {
			continue
		}
		// Don't need to add the keys as they are already in the edgeMap, we are just adding additional edges
		// between points.
		em.addLine(false, true, false, lines[i])
	}
}
*/

/*
func (em *EM) Triangulate() {
	triangulate.Triangulate(em)
}
*/

/*

func (em *EM) Triangulate2() {
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

		eq := NewEventQueue(lines)
		FindAllIntersectsWithEventQueueWithoutIntersectNotPolygon(eq, lines,
			func(src, dest int) bool {
				if src >= offset && dest >= offset {
					return true
				}
				if src < offset && skiplines[src] {
					return true
				}
				if dest < offset && skiplines[dest] {
					return true
				}
				return false
			},
			func(src, dest int) {
				if src < offset {
					// need to remove this possible edge.
					if dest >= offset {
						skiplines[src] = true
					}
					return
				}
				if dest < offset {
					// need to remove this possible edge.
					if src >= offset {
						skiplines[dest] = true
					}
					return
				}
			})
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
*/
func (em *EM) FindTriangles() (*maths.TriangleGraph, error) {

	//log.Println("Starting FindTriangles")
	//defer log.Println("Done with FindTriangles")

	type triEdge struct {
		edge        int
		tri         string
		constrained bool
	}
	var nodesToLabel []*maths.TriangleNode
	nodes := make(map[string]*maths.TriangleNode)
	seenPts := make(map[maths.Pt]bool)
	for i := range em.Keys {
		seenPts[em.Keys[i]] = true
		pts, ok := em.SubKeys(em.Keys[i])
		if !ok {
			// Should not happen
			continue
		}

		for j := range pts {
			if seenPts[pts[j]] {
				// Already processed this set.
				continue
			}
			tr1, tr2, err := em.TrianglesForEdge(em.Keys[i], pts[j])
			if err != nil {
				return nil, err
			}
			if tr1 == nil && tr2 == nil {
				// zero area triangle.
				// This can happend if an edge lays on the same line as another edge.
				continue
			}
			var trn1, trn2 *maths.TriangleNode

			if tr1 != nil {
				trn1, ok = nodes[tr1.Key()]
				if !ok {
					trn1 = &maths.TriangleNode{Triangle: *tr1}
					nodes[tr1.Key()] = trn1
				}
			}

			if tr2 != nil {
				trn2, ok = nodes[tr2.Key()]
				if !ok {
					trn2 = &maths.TriangleNode{Triangle: *tr2}
					nodes[tr2.Key()] = trn2
				}
			}

			if trn1 != nil && trn2 != nil {
				//log.Printf("len(nodesToLabel)=%v; Setting up Neighbors.", len(nodesToLabel)-1)
				edgeidx1 := trn1.EdgeIdx(em.Keys[i], pts[j])
				edgeidx2 := trn2.EdgeIdx(em.Keys[i], pts[j])
				constrained := em.Map[em.Keys[i]][pts[j]]
				trn1.Neighbors[edgeidx1] = maths.TriangleEdge{
					Node:          trn2,
					IsConstrained: constrained,
				}
				trn2.Neighbors[edgeidx2] = maths.TriangleEdge{
					Node:          trn1,
					IsConstrained: constrained,
				}
			}
			//log.Printf("tr1: %#v\n\tTr1:%#v\ntr2:%#v\n\tTr2:%#v", tr1, trn1, tr2, trn2)
			if em.BBox[0].IsEqual(em.Keys[i]) ||
				em.BBox[1].IsEqual(em.Keys[i]) ||
				em.BBox[2].IsEqual(em.Keys[i]) ||
				em.BBox[3].IsEqual(em.Keys[i]) {
				//log.Printf("BBox(%v %v %v %v) -- key[%v] %v", em.BBox[0], em.BBox[1], em.BBox[2], em.BBox[3], i, em.Keys[i])
				if trn1 != nil {
					nodesToLabel = append(nodesToLabel, trn1)
				}
				if trn2 != nil {
					nodesToLabel = append(nodesToLabel, trn2)
				}
			}
		}
	}
	currentLabel := maths.Outside
	var nextSetOfNodes []*maths.TriangleNode

	//log.Printf("Number of triangles found: %v", len(nodes))
	for len(nodesToLabel) > 0 {
		for i := range nodesToLabel {
			//log.Printf("Labeling node(%v of %v) as %v", i, len(nodesToLabel), currentLabel)
			nextSetOfNodes = append(nextSetOfNodes, nodesToLabel[i].LabelAs(currentLabel, false)...)
		}
		//log.Println("Next set of nodes:", nextSetOfNodes)
		nodesToLabel, nextSetOfNodes = nextSetOfNodes, nodesToLabel[:0]
		if currentLabel == maths.Outside {
			currentLabel = maths.Inside
		} else {
			currentLabel = maths.Outside
		}
	}
	var nodeSlice []*maths.TriangleNode
	for _, val := range nodes {
		nodeSlice = append(nodeSlice, val)
	}
	return maths.NewTriangleGraph(nodeSlice, em.BBox), nil
}
