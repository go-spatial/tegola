package validate

import (
	"context"
	"sync"

	"log"

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
			log.Println("Got error cleaning linestring", err)
			return p, err
		}
		cln, err = CleanCrossOvers(context.Background(), cln, 0)
		if err != nil {
			log.Println("Got error removing crossings", err)
			return p, err
		}

		p = append(p, basic.NewLine(cln...))
	}

	return p, nil
}

func CleanGeometry(g tegola.Geometry) (geo tegola.Geometry, err error) {
	if g == nil {
		return nil, nil
	}
	switch gg := g.(type) {

	case tegola.Polygon:

		return CleanPolygon(gg)
	case tegola.MultiPolygon:
		var mp basic.MultiPolygon
		for _, p := range gg.Polygons() {
			cp, err := CleanPolygon(p)
			if err != nil {
				return mp, err
			}
			mp = append(mp, cp)
		}
		return mp, nil
	}
	return g, nil
}
