package validate

import (
	"context"
	"fmt"
	"sync"

	"sort"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
)

// CleanLine will remove duplicate points, and points between the duplicate points. The exception to this, is the first and last points,
// are the same.
func CleanLine(g tegola.LineString) (l basic.Line, err error) {

	var ptsMap = make(map[maths.Pt][]int)
	var pts []maths.Pt
	for i, pt := range g.Subpoints() {

		p := maths.Pt{pt.X(), pt.Y()}
		ptsMap[p] = append(ptsMap[p], i)
		pts = append(pts, p)
	}

	for i := 0; i < len(pts); i++ {
		pt := pts[i]
		fpts := ptsMap[pt]
		l = append(l, basic.Point{pt.X, pt.Y})
		if len(fpts) > 1 {
			// we will need to skip a bunch of points.
			i = fpts[len(fpts)-1]
		}
	}
	return l, nil
}

func CleanLinestring(g []float64) (l []float64, err error) {

	var ptsMap = make(map[maths.Pt][]int)
	var pts []maths.Pt
	i := 0
	for x, y := 0, 1; y < len(g); x, y = x+2, y+2 {

		p := maths.Pt{g[x], g[y]}
		ptsMap[p] = append(ptsMap[p], i)
		pts = append(pts, p)
		i++
	}

	for i := 0; i < len(pts); i++ {
		pt := pts[i]
		fpts := ptsMap[pt]
		l = append(l, pt.X, pt.Y)
		if len(fpts) > 1 {
			// we will need to skip a bunch of points.
			i = fpts[len(fpts)-1]
		}
	}
	return l, nil
}

type crxPt struct {
	srcIdx  int
	destIdx int
	pt      maths.Pt
}

func cleanWorker(ctx context.Context, wg *sync.WaitGroup, idxChan <-chan int, crxChan chan<- crxPt, segs []maths.Line) {
	length := len(segs)
IdxLoop:
	for idx := range idxChan {
		if ctx.Err() != nil {
			break
		}
		line := segs[idx]
		// we need to skip the segment right next to us, as we share a pt.
		for i := idx + 2; i < length; i++ {
			if ctx.Err() != nil {
				break IdxLoop
			}
			if pt, ok := maths.Intersect(line, segs[i]); ok && line.InBetween(pt) && segs[i].InBetween(pt) {
				crxChan <- crxPt{
					srcIdx:  idx,
					destIdx: i,
					pt:      pt.Truncate(),
				}
			}
		}
	}
	wg.Done()
}

// CleanCrossOvers will iterate over each point pair looking for any lines that intersect with other segments in the linestring.
// If such a intersection is found, the intersection point will be inserted as the next point, and the points between the crossed line segments
// will be eliminated.
// This functions starts up goroutines, to stop work please pass a context in.
func CleanCrossOvers(ctx context.Context, g []float64, batchsize int) (l []float64, err error) {

	// First for each pair of points we need to create a point pair.
	segs, err := maths.NewSegments(g)
	if err != nil {
		return l, err
	}
	//log.Printf("Cleaning: segs length %v", len(segs))

	if IsSimple(segs) {
		return g, nil
	}

	intersectionMap := make(map[int]crxPt)

	maths.FindPolygonIntersects(segs, func(srcIdx, destIdx int, ptfn func() maths.Pt) bool {

		src := segs[srcIdx]
		dest := segs[destIdx]
		pt := ptfn()
		if !src.InBetween(pt) || !dest.InBetween(pt) { // ignore this intersection.
			return true
		}

		//log.Printf("Found intersection for (%v)[%v] -> (%v)[%v] @ %v", srcIdx, src, destIdx, dest, pt)

		intersectionMap[srcIdx] = crxPt{
			srcIdx:  srcIdx,
			destIdx: destIdx,
			pt:      pt.Truncate(),
		}
		if ctx.Err() != nil {
			return false
		}
		return true
	})

	if ctx.Err() != nil {
		return g, context.Canceled
	}

	// segment zero is special as it's endpt, startpt. We want to ignore this segment anyway.
	keys := make([]int, len(intersectionMap))
	{
		i := 0
		for k := range intersectionMap {
			keys[i] = k
			i++
		}
		sort.Ints(keys)
	}

	for i := 1; i < len(segs); {
		//log.Println(i, ":\tAdding:", segs[i][0].X, segs[i][0].Y)
		l = append(l, segs[i][0].X, segs[i][0].Y)
		item, ok := intersectionMap[i]
		if !ok { // segment does not intersect with anything. We can just add it to our line.
			i++
			continue
		}
		if segs[item.srcIdx][0].IsEqual(item.pt) || segs[item.destIdx][1].IsEqual(item.pt) {
			// skip dest completly.
			i = item.destIdx + 1
			continue
		}

		segs[item.destIdx][0].X = item.pt.X
		segs[item.destIdx][0].Y = item.pt.Y
		i = item.destIdx
	}
	l = append(l, segs[0][0].X, segs[0][0].Y)
	/*
		{
			lsegs, _ := maths.NewSegments(l)

			//CleanCrossOvers(ctx, l, batchsize+1)
			var simple bool = true

			log.Println("Testing new polygon: ", len(lsegs), "vs", len(segs))
			FindIntersects(lsegs, func(sidx, didx int, ptfn func() maths.Pt) bool {

				simple = false

					src := lsegs[sidx]
					dest := lsegs[didx]
					pt := ptfn()

						if !src.InBetween(pt) || !dest.InBetween(pt) { // ignore this intersection.
							log.Println("Is my simple function wrong?", src, dest, pt)
							return true
						}


				//	log.Printf("Found intersection for (%v)[%v] -> (%v)[%v] @ %v", sidx, src, didx, dest, pt.Truncate())
				return false
			})
			if !simple {
				log.Println("Not simple.")
			}

		}
	*/
	//log.Printf("Final line %#v", l)
	return l, nil
}

type SimplicityReason uint8

const (
	OuterRingNotClockwise         = SimplicityReason(1 << iota) // 1
	InnerRingsNotCounterClockwise                               // 2
	DuplicatePoints                                             // 4
	SelfIntersecting                                            // 8
	OtherError
)

// PolygonIsSimple will make sure that a polygon has the following properties satified..
// 1. The outer ring is clockwise and the interior rings are counterclockwise.
// 2. No ring has duplicate points.
// 3. No ring is self intersecting.
func PolygonIsSimple(g tegola.Polygon) (ok bool, reason SimplicityReason) {
	ok = true
MainLoop:
	for i, l := range g.Sublines() {
		// 0 is the outer ring.
		if i == 0 {
			if maths.WindingOrderOfLine(l) != maths.Clockwise {
				ok = false
				reason = reason | OuterRingNotClockwise
			}
		} else {
			// These are interior rings.
			if maths.WindingOrderOfLine(l) != maths.CounterClockwise {
				ok = false
				reason = reason | InnerRingsNotCounterClockwise
			}
		}
		// Sort the points in the line to make it easier to find dups.
		dupmap := make(map[string]struct{})
		for i, pt := range l.Subpoints() {
			_ = i
			key := fmt.Sprintf("%v,%v", pt.X(), pt.Y())
			if _, ok := dupmap[key]; ok {
				//log.Println("Found a Duplicate point at", i, "Pt", key)
				ok = false
				reason = reason | DuplicatePoints
				break MainLoop
			}
		}
		// Make sure there arn't any intersections.
		ppln := tegola.LineAsPointPairs(l)
		segs, err := maths.NewSegments(ppln)
		if err != nil {
			return false, reason | OtherError
		}
		if !IsSimple(segs) {
			ok = false
			reason = reason | SelfIntersecting
			break MainLoop
		}
	}
	return ok, reason
}

/*
func MultiPolygonIsSimple(g tegola.MultiPolygon) bool {
	for i, p := range g.Polygons() {
		if !PolygonIsSimple(p) {
			log.Printf("In Multi Polygon %#v:", g)
			log.Printf("Found Polygon(%v) to not be simple.", i)
			return false
		}
	}
	return true
}
*/

func LineStringToSegments(l tegola.LineString) ([]maths.Line, error) {
	ppln := tegola.LineAsPointPairs(l)
	return maths.NewSegments(ppln)
}
func FlipWindingOrderOfLine(l tegola.LineString) basic.Line {
	pts := l.Subpoints()
	bl := basic.Line{basic.Point{pts[0].X(), pts[0].Y()}}
	for i := len(pts) - 1; i > 0; i-- {
		bl = append(bl, basic.Point{pts[i].X(), pts[i].Y()})
	}
	return bl
}
func makePolygonValid(g tegola.Polygon) (mp basic.MultiPolygon, err error) {
	//log.Printf("Making Polygon valid\n%#v\n", g)
	var plygLines [][]maths.Line
	for _, l := range g.Sublines() {
		segs, err := LineStringToSegments(l)
		if err != nil {
			return mp, err
		}
		plygLines = append(plygLines, segs)
	}
	plyPoints, err := maths.MakeValid(plygLines...)
	if err != nil {
		return mp, err
	}
	for i := range plyPoints {
		// Each i is a polygon. Made up of line string points.
		var p basic.Polygon
		for j := range plyPoints[i] {
			// We need to transform plyPoints[i][j] into a basic.LineString.
			nl := basic.NewLineFromPt(plyPoints[i][j]...)
			if j == 0 {
				if nl.Direction() != maths.Clockwise {
					// We need to flip the line.
					nl = FlipWindingOrderOfLine(nl)
				}
			} else {
				if nl.Direction() != maths.CounterClockwise {
					// We need to flip the line.
					nl = FlipWindingOrderOfLine(nl)
				}
			}
			p = append(p, nl)
		}
		mp = append(mp, p)
	}
	return mp, err
}
func flipWindingOrderForPolygonLines(reason SimplicityReason, lines []tegola.LineString) (np basic.Polygon) {
	var lns basic.Polygon
	for i := range lines {
		lns = append(lns, basic.NewLineFromSubPoints(lines[i].Subpoints()...))
	}
	if reason&OuterRingNotClockwise == OuterRingNotClockwise {
		lns[0] = FlipWindingOrderOfLine(lns[0])
	}
	if len(lns) > 1 && reason&InnerRingsNotCounterClockwise == InnerRingsNotCounterClockwise {
		for i := range lns[1:] {
			if maths.WindingOrderOfLine(lns[i+1]) != maths.CounterClockwise {
				lns[i+1] = FlipWindingOrderOfLine(lns[i+1])
			}
		}
	}
	return lns
}
func MakePolygonValid(g tegola.Polygon) (mp basic.MultiPolygon, err error) {

	var reason SimplicityReason
	var ok bool
	if ok, reason = PolygonIsSimple(g); ok {
		return basic.NewMultiPolygonFromPolygons(g), nil
	}

	if (reason&DuplicatePoints == DuplicatePoints) || (reason&SelfIntersecting == SelfIntersecting) || (reason&OtherError == OtherError) {
		// Need to do the fix.
		return makePolygonValid(g)
	}
	mp = append(mp, flipWindingOrderForPolygonLines(reason, g.Sublines()))
	return mp, nil
}

func MakeMultiPolygonValid(g tegola.MultiPolygon) (mp basic.MultiPolygon, err error) {
	var reason SimplicityReason
	var ok bool
	var needToFix bool
	polygons := g.Polygons()
	appendedCount := 0
	for i := range polygons {
		if ok, reason = PolygonIsSimple(polygons[i]); ok {
			appendedCount++
			mp = append(mp, basic.NewPolygonFromSubLines(polygons[i].Sublines()...))
			continue
		}
		// if the polygon is not valid, depending on the reason we may be
		// able to fix it really quickly.
		// First check to see it's not a quick fix.
		if (reason&DuplicatePoints == DuplicatePoints) || (reason&SelfIntersecting == SelfIntersecting) || (reason&OtherError == OtherError) {
			//log.Println("Got a Major error", reason, appendedCount)
			needToFix = true
			break
		}
		if (reason&OuterRingNotClockwise == OuterRingNotClockwise) || (reason&InnerRingsNotCounterClockwise == InnerRingsNotCounterClockwise) {
			mp = append(mp, flipWindingOrderForPolygonLines(reason, polygons[i].Sublines()))
		}
	}
	if !needToFix {
		return mp, nil
	}
	// Repair will provide a new multipolygon.
	mp = mp[0:0]

	//log.Printf("[%v,%v,%v] Making MultiPolygon valid\n%#v\n", reason&SelfIntersecting, reason&OtherError, reason&OtherError, g)
	var plygLines [][]maths.Line
	for _, p := range g.Polygons() {
		for _, l := range p.Sublines() {
			segs, err := LineStringToSegments(l)
			if err != nil {
				return mp, err
			}
			plygLines = append(plygLines, segs)
		}
	}
	plyPoints, err := maths.MakeValid(plygLines...)
	//log.Printf("Got the following for MakeValid(\n%#v\n):\n%#v\n", plygLines, plyPoints)
	if err != nil {
		//log.Printf("MPolygon %#v", g)
		panic(fmt.Sprintln("Err", err))
		return mp, err
	}
	for i := range plyPoints {
		// Each i is a polygon. Made up of line string points.
		var p basic.Polygon
		for j := range plyPoints[i] {
			// We need to transform plyPoints[i][j] into a basic.LineString.
			nl := basic.NewLineFromPt(plyPoints[i][j]...)
			if j == 0 {
				if nl.Direction() != maths.Clockwise {
					// We need to flip the line.
					nl = FlipWindingOrderOfLine(nl)
				}
			} else {
				if nl.Direction() != maths.CounterClockwise {
					// We need to flip the line.
					nl = FlipWindingOrderOfLine(nl)
				}
			}
			p = append(p, nl)
		}
		mp = append(mp, p)
	}
	//log.Printf("Fixed:\n%#v\nTo:\n%#v\n", g, mp)
	return mp, err
}

func CleanPolygon(g tegola.Polygon) (p basic.Polygon, err error) {

	sublines := g.Sublines()
	for i, _ := range sublines {
		ln := sublines[i]
		ppln := tegola.LineAsPointPairs(ln)

		segs, err := maths.NewSegments(ppln)
		if err != nil {
			return p, err
		}

		if IsSimple(segs) { // No need to clean line.
			p = append(p, basic.NewLine(ppln...))
			continue
		}

		cln, err := CleanLinestring(ppln)
		if err != nil {
			//log.Println("Got error cleaning linestring", err)
			return p, err
		}
		cln, err = CleanCrossOvers(context.Background(), cln, 0)
		if err != nil {
			//log.Println("Got error removing crossings", err)
			return p, err
		}

		p = append(p, basic.NewLine(cln...))
	}

	return p, nil
}

func CleanGeometry(g tegola.Geometry) (geo tegola.Geometry, err error) {
	//return g, nil
	if g == nil {
		return nil, nil
	}
	switch gg := g.(type) {

	case tegola.Polygon:
		geo, err = MakePolygonValid(gg)
		return geo, err

	case tegola.MultiPolygon:
		geo, err = MakeMultiPolygonValid(gg)
		return geo, err
	}
	return g, nil
}
