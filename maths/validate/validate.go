package validate

import (
	"context"
	"sync"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
)

// CleanLinstring will remove duplicate points, and points between the duplicate points. The exception to this, is the first and last points,
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

	crxPtChan := make(chan crxPt)

	var wgProcessor sync.WaitGroup
	wgProcessor.Add(1)
	var intersectionMap = make(map[int]crxPt)
	go func() {
		// Keep working till the channel is closed.
		for item := range crxPtChan {
			intersectionMap[item.srcIdx] = item
		}
		wgProcessor.Done()
	}()

	var wgWorker sync.WaitGroup
	if batchsize <= 0 {
		batchsize = 100
	}

	if batchsize > len(segs) {
		batchsize = 1
		if len(segs) > 20 {
			batchsize = len(segs) - 10
		}
	}

	idxChan := make(chan int, batchsize)
	wgWorker.Add(batchsize)
	for i := 0; i < batchsize; i++ {
		go cleanWorker(ctx, &wgWorker, idxChan, crxPtChan, segs)
	}
	for i := 0; i < len(segs)-1; i++ {
		idxChan <- i
	}
	close(idxChan)
	wgWorker.Wait()
	close(crxPtChan)
	wgProcessor.Wait()

	if ctx.Err() != nil {
		return g, context.Canceled
	}

	// segment zero is special as it's endpt, startpt. We want to ignore this segment anyway.

	startIdx := 1
	if item, ok := intersectionMap[startIdx]; ok {
		//log.Printf("index: %v Line:%v,ok:%v,item:%v", 1, segs[startIdx], ok, item)
		if item2, ok := intersectionMap[item.destIdx]; ok {
			//log.Printf("index: %v Line:%v,ok:%v,item:%v", item.destIdx, segs[item.destIdx], ok, item2)
			// we need to modify the map value.
			item2.pt = item.pt
			startIdx = item.destIdx - 1
		} else {
			//log.Printf("index: %v Line:%v,ok:%v,item:%v", item.destIdx, segs[item.destIdx], ok, item2)
			l = append(l, item.pt.X, item.pt.Y)
			startIdx = item.destIdx
		}
	}
	for i := startIdx; i < len(segs); i++ {

		if ctx.Err() != nil {
			return l, context.Canceled
		}

		line := segs[i]
		item, ok := intersectionMap[i]
		//log.Printf("index: %v Line:%v,ok:%v,item:%v", i, line, ok, item)
		l = append(l, line[0].X, line[0].Y)
		if !ok {
			// Both points are good.
			continue
		}
		if _, ok := intersectionMap[item.destIdx]; ok {

			// modify the start point of the segment.
			segs[item.destIdx][0] = item.pt
			i = item.destIdx - 1
			continue
		}
		l = append(l, item.pt.X, item.pt.Y)
		i = item.destIdx

	}
	l = append(l, segs[0][0].X, segs[0][0].Y)
	//log.Printf("Final line %#v", l)
	return l, nil
}

func CleanPolygon(g tegola.Polygon) (p basic.Polygon, err error) {

	for _, ln := range g.Sublines() {
		ppln := tegola.LineAsPointPairs(ln)
		cln, err := CleanLinestring(ppln)
		if err != nil {
			return p, err
		}
		cln, err = CleanCrossOvers(context.Background(), cln, 10)
		if err != nil {
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
