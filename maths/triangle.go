package maths

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"time"
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

type Triangle [3]Pt

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
}

func (t *Triangle) FindEdge(e Line) (idx int, err error) {
	switch {
	case e[0].IsEqual(t[0]) && e[1].IsEqual(t[1]):
		return 0, nil
	case e[0].IsEqual(t[1]) && e[1].IsEqual(t[0]):
		return 0, nil
	case e[0].IsEqual(t[1]) && e[1].IsEqual(t[2]):
		return 1, nil
	case e[0].IsEqual(t[2]) && e[1].IsEqual(t[1]):
		return 1, nil
	case e[0].IsEqual(t[2]) && e[1].IsEqual(t[0]):
		return 2, nil
	case e[0].IsEqual(t[0]) && e[1].IsEqual(t[2]):
		return 2, nil
	}

	return -1, errors.New("Edge not on triangle.")

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
func (t *Triangle) LREdge(n int) Line {
	if n < 0 || n == 0 {
		return Line{t[0], t[1]}
	}
	if n > 2 || n == 2 {
		return Line{t[0], t[2]}
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

func (t *Triangle) LREdges() [3]Line {
	return [3]Line{
		{t[0], t[1]},
		{t[1], t[2]},
		{t[0], t[2]},
	}
}

func (t *Triangle) EdgeIdx(pt1, pt2 Pt) int {
	if t == nil {
		return -1
	}
	if pt1.IsEqual(t[0]) {
		if pt2.IsEqual(t[1]) {
			return 0
		}
		if pt2.IsEqual(t[2]) {
			return 2
		}
		return -1
	}
	if pt2.IsEqual(t[0]) {
		if pt1.IsEqual(t[1]) {
			return 0
		}
		if pt1.IsEqual(t[2]) {
			return 2
		}
		return -1
	}
	if pt1.IsEqual(t[1]) {
		if pt2.IsEqual(t[2]) {
			return 1
		}
		return -1
	}
	if pt2.IsEqual(t[1]) {
		if pt1.IsEqual(t[2]) {
			return 1
		}
		return -1
	}
	return -1
}

func (t *Triangle) Key() string {
	if t == nil {
		return ""
	}
	sort.Sort(t)
	return fmt.Sprintf("(%v %v %v)", t[0], t[1], t[2])
}

func (t *Triangle) Points() []Pt { return []Pt{t[0], t[1], t[2]} }
func (t *Triangle) Point(i int) Pt {
	switch {
	case i <= 0:
		return t[0]
	case i >= 2:
		return t[2]
	default:
		return t[1]
	}
}
func (t *Triangle) Len() int {
	if t == nil {
		return 0
	}
	return len(t)
}

// If t is nil, we want to panic, as this this a programming bug.
func (t *Triangle) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t *Triangle) Less(i, j int) bool { return XYOrder(t[i], t[j]) == -1 }

func (t *Triangle) Equal(t1 *Triangle) bool {
	if t == nil || t1 == nil {
		return t == t1
	}
	sort.Sort(t)
	sort.Sort(t1)
	return t[0].IsEqual(t1[0]) && t[1].IsEqual(t1[1]) && t[2].IsEqual(t1[2])
}

func (t *Triangle) EqualAnyPt(pts ...Pt) bool {
	if t == nil {
		return false
	}
	for _, pt := range pts {
		if pt.IsEqual(t[0]) || pt.IsEqual(t[1]) || pt.IsEqual(t[2]) {
			return true
		}
	}
	return false
}

func AreaOfTriangle(v0, v1, v2 Pt) float64 {
	d := (v0.X * (v1.Y - v2.Y)) + (v1.X * (v2.Y - v0.Y)) + (v2.X * (v0.Y - v1.Y))
	a := d / 2
	return a
}

func (t *Triangle) Area() float64 {
	if t == nil {
		return 0
	}
	area := AreaOfTriangle(t[0], t[1], t[2])
	if area < 0 {
		return 0 - area
	}
	return area
}

func (t *Triangle) Center() Pt {
	if t == nil {
		return Pt{0, 0}
	}
	return Pt{
		X: (t[0].X + t[1].X + t[2].X) / 3,
		Y: (t[0].Y + t[1].Y + t[2].Y) / 3,
	}
}

// Will create a new Triangle and sort the points.k
func NewTriangle(pt1, pt2, pt3 Pt) (tri Triangle) {
	if pt1 == pt2 || pt1 == pt3 || pt2 == pt3 {
		panic(fmt.Sprintf("All three points of a triangle must be different. < %v , %v , %v >", pt1, pt2, pt3))
	}
	tri = Triangle{pt1, pt2, pt3}
	sort.Sort(&tri)
	return tri
}

type TriangleEdge struct {
	Node          *TriangleNode
	IsConstrained bool
}

func (te *TriangleEdge) Dump() {
	if te == nil || te.Node == nil {
		// log.Println("Triangle: nil")
		return
	}
	//log.Println("Triangle: ", te.Node.Key(), "Constrained:", te.IsConstrained)
}

// label is the he label for the triangle. Is in "inside" or "outside".
// TODO: gdey — would be make more sense to just have a bool here? IsInside or somthing like that?
type Label uint8

func (l Label) String() string {
	switch l {
	case Outside:
		return "outside"
	case Inside:
		return "inside"
	default:
		return "unknown"
	}
}

const (
	Unknown Label = iota
	Outside
	Inside
)

type TriangleNode struct {
	Triangle
	// Edge 0 has pt's 0 and 1.
	// Edge 1 has pt's 1 and 2.
	// Edge 2 has pt's 0 and 2.
	Neighbors [3]TriangleEdge
	Label     Label
}

func (tn *TriangleNode) Dump() {
	if tn == nil {
		//log.Println("Tringle: nil")
		return
	}
	//log.Println("Triangle:", tn.Triangle.Key(), "label:", tn.Label)
	for i, val := range tn.Neighbors {
		_ = i
		if val.Node == nil {
			//log.Printf("\t %v :  nil", i)
			continue
		}
		//log.Printf("\t %v : %v \t Constrained: %v", i, val.Node.Key(), val.IsConstrained)
	}
}

func (tn *TriangleNode) LabelAs(l Label, force bool) (unlabled []*TriangleNode) {
	if tn == nil {
		return unlabled
	}
	if !force && tn.Label != Unknown {
		//log.Println("Skipping labeling", force, tn.Label)
		return unlabled
	}
	tn.Label = l
	//log.Printf("Labeling as %v: Neighbors are %#v", l, tn.Neighbors)
	for i := range tn.Neighbors {
		//log.Println("Unlabed:", unlabled)
		if tn.Neighbors[i].IsConstrained {
			// We add this edge to our unlabled array.
			unlabled = append(unlabled, tn.Neighbors[i].Node)
			continue
		}
		unlabled = append(unlabled, tn.Neighbors[i].Node.LabelAs(l, false)...)
	}
	//log.Println("Unlabed:", unlabled)
	return unlabled
}

type TriangleGraph struct {
	triangles []*TriangleNode
	// List of the triangles that are labelled as outside.
	outside []int
	// List of the triangles that are labelled as inside.
	inside   []int
	bounding []int
}

func (tg *TriangleGraph) Triangles() []*TriangleNode {
	return tg.triangles
}

func (tg *TriangleGraph) TrianglesAsMP() (mp [][][]Pt) {
	for i := range tg.triangles {
		if tg.triangles[i] == nil {
			continue
		}
		mp = append(mp, [][]Pt{tg.triangles[i].Triangle[:]})
	}
	return mp
}

func (tg *TriangleGraph) Inside() []*TriangleNode {
	r := make([]*TriangleNode, 0, len(tg.inside))
	for _, i := range tg.inside {
		r = append(r, tg.triangles[i])
	}
	return r
}
func (tg *TriangleGraph) Outside() []*TriangleNode {
	r := make([]*TriangleNode, 0, len(tg.outside))
	for _, i := range tg.outside {
		r = append(r, tg.triangles[i])
	}
	return r
}

func simplifyNumberOfLines(lines []Line) (sln []Line) {
	var m1, m2 float64
	var ok1, ok2 bool
	if len(lines) <= 2 {
		return lines
	}

	lineToAdd := lines[len(lines)-1]
	m1, _, ok1 = lineToAdd.SlopeIntercept()
	for i := 0; i < len(lines); i, m1, ok1 = i+1, m2, ok2 {
		m2, _, ok2 = lines[i].SlopeIntercept()
		if m1 != m2 || ok1 != ok2 {
			sln = append(sln, lineToAdd)
			lineToAdd = lines[i]
			continue
		}
		switch {
		case lineToAdd[0].IsEqual(lines[i][0]):
			lineToAdd[0] = lines[i][1]
		case lineToAdd[0].IsEqual(lines[i][1]):
			lineToAdd[0] = lines[i][0]
		case lineToAdd[1].IsEqual(lines[i][0]):
			lineToAdd[1] = lines[i][0]
		case lineToAdd[1].IsEqual(lines[i][1]):
			lineToAdd[1] = lines[i][1]
		case lineToAdd[1].IsEqual(lines[i][0]):
		default:
			sln = append(sln, lineToAdd)
			lineToAdd = lines[i]

		}

	}
	sln = append(sln, lineToAdd)

	return sln

}

func (tg *TriangleGraph) Rings() (rings [][]Line) {

	if tg == nil {
		panic("TG nil!")
		return rings
	}

	//log.Println("Starting TriangleGraph Rings")
	//defer log.Println("Done with TriangleGraph Rings")
	// the key is the index of the trianlgle in the graph array.
	seen := make(map[string]struct{})
	// We are going to walk all triangles that are labeled as inside nodes, this will generate a set
	// of segments. The sements may form one or more close rings. We want to be greedy in the consumption of
	// these segments when reassembling the segments into rings.

	//log.Println("Going through inside.", tg.inside)
	for _, i := range tg.inside {
		//log.Printf("Looking at tg.triangle[%v].Key(%v)", i, tg.triangles[i].Key())
		if _, ok := seen[tg.triangles[i].Key()]; ok {
			//log.Println("Skipping ", i, tg.triangles[i].Key())
			continue
		}
		nodesToProcess := []*TriangleNode{tg.triangles[i]}
		var linesToProcess []Line

		for offset := 0; offset != len(nodesToProcess); offset++ {
			node := nodesToProcess[offset]
			if _, ok := seen[node.Key()]; ok {
				//log.Println("Skipping Node ", offset, node.Key())
				continue
			}
			seen[node.Key()] = struct{}{}
			for j := range node.Neighbors {
				if node.Neighbors[j].Node != nil && node.Neighbors[j].Node.Label == node.Label {
					nodesToProcess = append(nodesToProcess, node.Neighbors[j].Node)
				} else {
					//log.Println("Found edge to process: ", node.LREdge(j))
					linesToProcess = append(linesToProcess, node.LREdge(j))
				}
			}
		}
		// Now need to deal with Lines.
		//log.Println("Added lines to ring.", linesToProcess)
		if len(linesToProcess) > 0 {
			rings = append(rings, linesToProcess)
		}

	}
	return rings
}

func NewTriangleGraph(tri []*TriangleNode, bbox [4]Pt) (tg *TriangleGraph) {
	tg = &TriangleGraph{triangles: tri}
	for i := range tg.triangles {
		switch tg.triangles[i].Label {
		case Inside:
			tg.inside = append(tg.inside, i)
		case Outside:
			tg.outside = append(tg.outside, i)
			if tg.triangles[i].EqualAnyPt(bbox[0], bbox[1], bbox[2], bbox[3]) {
				tg.bounding = append(tg.bounding, i)
			}
		}
	}
	return tg
}

type ByXY []Pt

func (t ByXY) Len() int           { return len(t) }
func (t ByXY) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t ByXY) Less(i, j int) bool { return XYOrder(t[i], t[j]) == -1 }

type ByXYLine []Line

func (t ByXYLine) Len() int      { return len(t) }
func (t ByXYLine) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t ByXYLine) Less(i, j int) bool {
	li, lj := t[i].LeftRightMostAsLine(), t[j].LeftRightMostAsLine()
	ret := XYOrder(li[0], lj[0])
	if ret == 0 {
		ret = XYOrder(li[1], lj[1])
	}
	return ret == -1
}

func PointPairs(pts []Pt) ([][2]Pt, error) {
	if len(pts) <= 1 {
		return nil, fmt.Errorf("Not enough pts to make pairs.")
	}
	n := len(pts)
	switch n {

	case 2:
		return [][2]Pt{
			{pts[0], pts[1]},
		}, nil
	case 3:
		return [][2]Pt{
			{pts[0], pts[1]},
			{pts[0], pts[2]},
			{pts[1], pts[2]},
		}, nil
	case 4:
		return [][2]Pt{
			{pts[0], pts[1]},
			{pts[0], pts[2]},
			{pts[0], pts[3]},
			{pts[1], pts[2]},
			{pts[1], pts[3]},
			{pts[2], pts[3]},
		}, nil

	default:

		ret := make([][2]Pt, n*(n-1)/2)
		c := 0
		for i := 0; i < n-1; i++ {
			for j := i + 1; j < n; j++ {
				ret[c][0], ret[c][1] = pts[i], pts[j]
				c++
			}
		}
		return ret, nil
	}
}

// insureConnected will add a connecting line as needed to the given polygons. If there is only one line in a polygon, it will be left alone.
func insureConnected(polygons ...[]Line) (ret [][]Line) {
	ret = make([][]Line, len(polygons))
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
			ret[i] = append(ret[i], Line{polygons[i][ln-1][1], polygons[i][0][0]})
		}
	}
	return ret
}

// desctucture will split the given polygons into their composite lines, breaking up lines at intersection points. It will remove lines that overlap as well. Polygons need to be fully connected before calling this function.
func destructure(polygons [][]Line) (lines []Line) {

	// First we need to combine all the segments.
	var segments []Line
	{
		segs := make(map[Line]struct{})
		for i := range polygons {
			for _, ln := range polygons[i] {
				segs[ln.LeftRightMostAsLine()] = struct{}{}
			}
		}
		for ln := range segs {
			segments = append(segments, ln)
		}
		if len(segments) <= 1 {
			return segments
		}
		sort.Sort(ByXYLine(segments))
	}

	// linesToSplit holds a list of points for that segment to be split at. This list will have to be
	// ordered and deuped.
	splitPts := make([][]Pt, len(segments))

	FindIntersects(segments, func(src, dest int, ptfn func() Pt) bool {

		sline, dline := segments[src], segments[dest]

		pt := ptfn() // left most point.
		pt.X = float64(int64(pt.X))
		pt.Y = float64(int64(pt.Y))
		if !(pt.IsEqual(sline[0]) || pt.IsEqual(sline[1])) {
			splitPts[src] = append(splitPts[src], pt)
		}
		if !(pt.IsEqual(dline[0]) || pt.IsEqual(dline[1])) {
			splitPts[dest] = append(splitPts[dest], pt)
		}
		return true
	})

	for i := range segments {
		if splitPts[i] == nil {
			lines = append(lines, segments[i].LeftRightMostAsLine())
			continue
		}
		sort.Sort(ByXY(splitPts[i]))
		lidx, ridx := Line(segments[i]).XYOrderedPtsIdx()
		lpt, rpt := segments[i][lidx], segments[i][ridx]
		for j := range splitPts[i] {
			if lpt.IsEqual(splitPts[i][j]) {
				// Skipp dups.
				continue
			}
			lines = append(lines, Line{lpt, splitPts[i][j]}.LeftRightMostAsLine())
			lpt = splitPts[i][j]
		}
		if !lpt.IsEqual(rpt) {
			lines = append(lines, Line{lpt, rpt}.LeftRightMostAsLine())
		}
	}

	sort.Sort(ByXYLine(lines))
	return lines
}

// We only care about the first triangle node, as an edge can only contain two triangles.
type aNodeList map[[2]Pt]*TriangleNode

// AddTrinagleForPts will order the points, create a new Triangle and add it to the Node List.
func (nl aNodeList) AddTriangleForPts(pt1, pt2, pt3 Pt, fnIsConstrained func(pt1, pt2 Pt) bool) (tri *TriangleNode, err error) {

	var fn = func(pt1, pt2 Pt) bool { return false }

	if fnIsConstrained != nil {
		fn = fnIsConstrained
	}

	tri = &TriangleNode{Triangle: NewTriangle(pt1, pt2, pt3)}

	for i := range tri.Points() {
		j := i + 1
		if i == 2 {
			j = 0
		}
		edge := [2]Pt{tri.Point(i), tri.Point(j)}
		tri.Neighbors[i].IsConstrained = fn(edge[0], edge[1])

		node, ok := nl[edge]
		if !ok {
			nl[edge] = tri
			continue
		}

		tri.Neighbors[i].Node = node
		// Find the point idx so we know which slot we need to add ourself to.
		for k, pt := range node.Triangle {
			if !pt.IsEqual(edge[1]) {
				continue
			}
			if node.Neighbors[k].Node != nil {
				// return an error
				//log.Printf("More then two triangles are sharing an edge. \n\t%+v\n\t%v\n\t%+v\n\t %v %v %v", node, k, node.Neighbors[k].Node, pt1, pt2, pt3)
				panic("More then two triangles are sharing an edge.")
				return nil, fmt.Errorf("More then two triangles are sharing an edge. \n\t%+v\n\t%v\n\t%+v\n\t %v %v %v", node, k, node.Neighbors[k].Node, pt1, pt2, pt3)
			}
			// Assign ourself to the Neighbor's correct slot.
			node.Neighbors[k].Node = tri
		}
	}
	return tri, nil
}

type EdgeMap struct {
	Keys         []Pt
	Map          map[Pt]map[Pt]bool
	Segments     []Line
	BBox         [4]Pt
	destructured []Line
}

func (em *EdgeMap) SubKeys(pt Pt) (skeys []Pt, ok bool) {
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
	sort.Sort(ByXY(skeys))
	return skeys, ok
}

func (em *EdgeMap) trianglesForEdge(pt1, pt2 Pt) (*Triangle, *Triangle, error) {

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
	var triangles [2]*Triangle
NextApts:
	for i := range apts {
	NextBpts:
		for j := range bpts {
			if apts[i].IsEqual(bpts[j]) {
				tri := NewTriangle(pt1, pt2, apts[i])
				area := AreaOfTriangle(pt1, pt2, apts[i])
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

func generateEdgeMap(destructuredLines []Line) (em EdgeMap) {
	em.destructured = destructuredLines
	em.Map = make(map[Pt]map[Pt]bool)
	em.Segments = make([]Line, 0, len(destructuredLines))

	// First we need to combine all the segments.
	var minPt, maxPt Pt
	for i := range destructuredLines {
		seg := destructuredLines[i]
		pt1, pt2 := seg[0], seg[1]
		if pt1.IsEqual(pt2) {
			continue // skip point lines
		}
		if _, ok := em.Map[pt1]; !ok {
			em.Map[pt1] = make(map[Pt]bool)
		}
		if _, ok := em.Map[pt2]; !ok {
			em.Map[pt2] = make(map[Pt]bool)
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
	// The adjustBBoxBy is just an arbitrary number, we just care that is out further then the largest and smallest points.
	minPt.X, minPt.Y, maxPt.X, maxPt.Y = minPt.X-adjustBBoxBy, minPt.Y-adjustBBoxBy, maxPt.X+adjustBBoxBy, maxPt.Y+adjustBBoxBy
	bbv1, bbv2, bbv3, bbv4 := minPt, Pt{maxPt.X, minPt.Y}, maxPt, Pt{minPt.X, maxPt.Y}

	// We want our bound box to be in a known position. so that we can able things correctly.
	em.Segments = append([]Line{
		{bbv1, bbv2}, // Top edge
		{bbv2, bbv3}, // right edge
		{bbv3, bbv4}, // bottom edge
		{bbv4, bbv1}, // left edge
	}, em.Segments...)
	em.BBox = [4]Pt{bbv1, bbv2, bbv3, bbv4}
	em.addLine(false, false, true, em.Segments[0:4]...)

	keys := em.Keys
	sort.Sort(ByXY(keys))
	em.Keys = []Pt{keys[0]}
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

func (em *EdgeMap) addLine(constrained bool, addSegments bool, addKeys bool, lines ...Line) {
	if em == nil {
		return
	}
	if em.Map == nil {
		em.Map = make(map[Pt]map[Pt]bool)
	}
	for _, l := range lines {
		pt1, pt2 := l[0], l[1]
		if _, ok := em.Map[pt1]; !ok {
			em.Map[pt1] = make(map[Pt]bool)
		}
		if _, ok := em.Map[l[1]]; !ok {
			em.Map[pt2] = make(map[Pt]bool)
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

func (em *EdgeMap) Dump() {
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

func (em *EdgeMap) Triangulate1() {

	//defer log.Println("Done with Triangulate")
	keys := em.Keys

	//log.Println("Starting to Triangulate. Keys", len(keys))
	// We want to run through all the keys generating possible edges, and then
	// collecting the ones that don't intersect with the edges in the map already.
	var lines = make([]Line, 0, 2*len(keys))
	stime := time.Now()
	for i := 0; i < len(keys)-1; i++ {
		lookup := em.Map[keys[i]]
		//log.Println("Looking at i", i, "Lookup", lookup)
		for j := i + 1; j < len(keys); j++ {
			if _, ok := lookup[keys[j]]; ok {
				// Already have an edge with this point
				continue
			}
			l := Line{keys[i], keys[j]}
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

			/*
				if src >= offset && dest >= offset {
					return
				}
				//these are two Segments intersecting with each other ignore.

				if src < offset && dest < offset {
					skiplines[dest] = true
					return
				}
			*/
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

func (em *EdgeMap) Triangulate() {
	//defer log.Println("Done with Triangulate")
	keys := em.Keys
	lnkeys := len(keys) - 1
	//log.Println("Starting to Triangulate. Keys", len(keys))
	// We want to run through all the keys generating possible edges, and then
	// collecting the ones that don't intersect with the edges in the map already.
	for i := 0; i < lnkeys; i++ {
		lookup := em.Map[keys[i]]
		var possibleEdges []Line
		for j := i + 1; j < len(keys); j++ {
			if _, ok := lookup[keys[j]]; ok {
				// Already have an edge with this point
				continue
			}
			l := Line{keys[i], keys[j]}
			possibleEdges = append(possibleEdges, l)
		}

		// Now we need to do a line sweep to see which of the possible edges we want to keep.
		lines := append([]Line{}, possibleEdges...)
		offset := len(lines)
		lines = append(lines, em.Segments...)
		skiplines := make([]bool, offset)

		/*
			stime := time.Now()
		*/
		eq := NewEventQueue(lines)
		/*
			etime := time.Now()
			log.Println(i, "of", lnkeys, "building event queue took: ", etime.Sub(stime))
			stime = etime
		*/
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
		/*
			etime = time.Now()
			log.Println(i, "of", lnkeys, "Find Intersects took: ", etime.Sub(stime))
		*/
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
func (em *EdgeMap) FindTriangles() (*TriangleGraph, error) {
	//log.Println("Starting FindTriangles")
	//defer log.Println("Done with FindTriangles")

	type triEdge struct {
		edge        int
		tri         string
		constrained bool
	}
	var nodesToLabel []*TriangleNode
	nodes := make(map[string]*TriangleNode)
	seenPts := make(map[Pt]bool)
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
			tr1, tr2, err := em.trianglesForEdge(em.Keys[i], pts[j])
			if err != nil {
				return nil, err
			}
			if tr1 == nil && tr2 == nil {
				// zero area triangle.
				// This can happen if an edge lays on the same line as another edge.
				continue
			}
			var trn1, trn2 *TriangleNode

			if tr1 != nil {
				trn1, ok = nodes[tr1.Key()]
				if !ok {
					trn1 = &TriangleNode{Triangle: *tr1}
					nodes[tr1.Key()] = trn1
				}
			}

			if tr2 != nil {
				trn2, ok = nodes[tr2.Key()]
				if !ok {
					trn2 = &TriangleNode{Triangle: *tr2}
					nodes[tr2.Key()] = trn2
				}
			}

			if trn1 != nil && trn2 != nil {
				//log.Printf("len(nodesToLabel)=%v; Setting up Neighbors.", len(nodesToLabel)-1)
				edgeidx1 := trn1.EdgeIdx(em.Keys[i], pts[j])
				edgeidx2 := trn2.EdgeIdx(em.Keys[i], pts[j])
				constrained := em.Map[em.Keys[i]][pts[j]]
				trn1.Neighbors[edgeidx1] = TriangleEdge{
					Node:          trn2,
					IsConstrained: constrained,
				}
				trn2.Neighbors[edgeidx2] = TriangleEdge{
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
	currentLabel := Outside
	var nextSetOfNodes []*TriangleNode

	//log.Printf("Number of triangles found: %v", len(nodes))
	for len(nodesToLabel) > 0 {
		for i := range nodesToLabel {
			//log.Printf("Labeling node(%v of %v) as %v", i, len(nodesToLabel), currentLabel)
			nextSetOfNodes = append(nextSetOfNodes, nodesToLabel[i].LabelAs(currentLabel, false)...)
		}
		//log.Println("Next set of nodes:", nextSetOfNodes)
		nodesToLabel, nextSetOfNodes = nextSetOfNodes, nodesToLabel[:0]
		if currentLabel == Outside {
			currentLabel = Inside
		} else {
			currentLabel = Outside
		}
	}
	var nodeSlice []*TriangleNode
	for _, val := range nodes {
		nodeSlice = append(nodeSlice, val)
	}
	return NewTriangleGraph(nodeSlice, em.BBox), nil
}

// makeValid takes a set of polygons that is invalid,
// will include triangles outside of the polygons provided, creating a convex hull.
func makeValid(plygs ...[]Line) (polygons [][][]Pt, err error) {
	//defer trace(fmt.Sprintf("makeValid(%v --\n%#v\n): ", len(plygs), plygs))()

	//stime := time.Now()
	destructuredLines := destructure(insureConnected(plygs...))
	/*
		etime := time.Now()
		log.Println("dstructedLines took: ", etime.Sub(stime))
		stime = etime
	*/
	edgeMap := generateEdgeMap(destructuredLines)
	/*
		etime = time.Now()
		log.Println("generateEdgeMap took: ", etime.Sub(stime))
		stime = etime
	*/
	edgeMap.Triangulate()
	/*
		etime = time.Now()
		log.Println("Triangulate took: ", etime.Sub(stime))
		stime = etime
	*/
	triangleGraph, err := edgeMap.FindTriangles()
	if err != nil {
		return polygons, err
	}
	/*
		etime = time.Now()
		log.Println("Find Triangles took: ", etime.Sub(stime))
		stime = etime
	*/
	rings := triangleGraph.Rings()
	for _, ring := range rings {
		polygon := constructPolygon(ring)
		polygons = append(polygons, polygon)
	}
	/*
		etime = time.Now()
		log.Println("Rings and ConstructPolygon took: ", etime.Sub(stime))
	*/
	// Need to sort the polygons in the multipolygon to get a consistent order.
	sort.Sort(plygByFirstPt(polygons))
	return polygons, nil
}

func MakeValid(plygs ...[]Line) (polygons [][][]Pt, err error) {
	return makeValid(plygs...)
}

type byArea [][]Pt

func (r byArea) Less(i, j int) bool {
	iarea := AreaOfRing(r[i]...)
	jarea := AreaOfRing(r[j]...)
	return iarea < jarea
}
func (r byArea) Len() int {
	return len(r)
}
func (r byArea) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type plygByFirstPt [][][]Pt

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
	return XYOrder(p1[0][0], p2[0][0]) == -1
}

func (p plygByFirstPt) Len() int {
	return len(p)
}
func (p plygByFirstPt) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func constructPolygon(lines []Line) [][]Pt {
	// We sort the lines, for a couple of reasons.
	// The first is because the smallest and largest lines are going to be part of the external ring.
	// The second by sorting it moves lines that are connected closer together.
	sort.Sort(ByXYLine(lines))
	rings := [][]Pt{{lines[0][0], lines[0][1]}}
	closed := make(map[int]bool)
NextLine:
	for _, line := range lines[1:] {

		for i, ring := range rings {
			if closed[i] {
				continue
			}
			end := len(ring) - 1
			switch {
			case ring[0].IsEqual(line[0]):
				if !ring[end].IsEqual(line[1]) {
					rings[i] = append([]Pt{line[1]}, ring...)
				} else {
					closed[i] = true
				}
				continue NextLine
			case ring[0].IsEqual(line[1]):
				if !ring[end].IsEqual(line[0]) {
					rings[i] = append([]Pt{line[0]}, ring...)
				} else {
					closed[i] = true
				}
				continue NextLine
			case ring[end].IsEqual(line[0]):
				if !ring[0].IsEqual(line[1]) {
					rings[i] = append(rings[i], line[1])
				} else {
					closed[i] = true
				}
				continue NextLine
			case ring[end].IsEqual(line[1]):
				if !ring[0].IsEqual(line[0]) {
					rings[i] = append(rings[i], line[0])
				} else {
					closed[i] = true
				}
				continue NextLine
			}
		}
		// Need to add it to a new ring.
		rings = append(rings, []Pt{line[0], line[1]})
	}

	for i, ring := range rings {
		minidx := 0
		for j := 1; j < len(ring); j++ {
			if XYOrder(ring[minidx], ring[j]) == 1 {
				minidx = j
			}
		}
		if minidx != 0 {
			rings[i] = append(ring[minidx:], ring[:minidx]...)
		}
	}
	// Need to sort the rings by size. The largest ring by area needs to be the first ring.

	sort.Sort(byArea(rings))
	return rings
}

type PointNode struct {
	Pt   Pt
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
