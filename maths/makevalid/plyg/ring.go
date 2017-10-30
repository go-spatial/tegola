package plyg

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	svg "github.com/ajstarks/svgo"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/points"
)

type Ring struct {
	Points []maths.Pt
	Label  maths.Label
	hasBB  bool
	bb     [4]float64
}

func (r *Ring) BBox() [4]float64 {
	if r.hasBB {
		return r.bb
	}
	r.bb = points.BBox(r.Points)
	r.hasBB = true
	return r.bb
}

// LineRing returns a copy of the points in the correct winding order.
func (r Ring) LineRing() (pts []maths.Pt) {
	pts = append(pts, r.Points...)
	wo := maths.WindingOrderOfPts(pts)
	if (r.Label == maths.Inside && wo == maths.CounterClockwise) ||
		(r.Label != maths.Inside && wo == maths.Clockwise) {
		points.Reverse(pts)
	}
	// Lets move the points around so that the left-top most point is first.
	points.RotateToLowestsFirst(pts)
	return pts
}

type RingDesc struct {
	Idx   int
	PtIdx int
	Label maths.Label
}

type YEdge struct {
	// Start y value (lowest value) of the edge.
	Y     float64
	Descs []RingDesc
}

type EdgeByY []YEdge

func (s EdgeByY) Len() int           { return len(s) }
func (s EdgeByY) Less(i, j int) bool { return s[i].Y < s[j].Y }
func (s EdgeByY) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type RingCol struct {
	Rings       []Ring
	X1, X2      float64
	Y1s         []YEdge
	Y2s         []YEdge
	foundInside bool
}

func (rc *RingCol) String() string {
	if rc == nil {
		return "Ring: nil"
	}
	s := fmt.Sprintf("Ring: x1(%v), x2(%v)\n\tRings(%v){", rc.X1, rc.X2, len(rc.Rings))
	for i, r := range rc.Rings {
		s += fmt.Sprintf("\n\t\t(%v):%v", i, r)
	}
	if len(rc.Rings) > 0 {
		s += "\n\t"
	}
	s += "}\n"
	s += fmt.Sprintf("\n\tY1s(%v){", len(rc.Y1s))
	for i, r := range rc.Y1s {
		s += fmt.Sprintf("\n\t\t(%v):%v", i, r)
	}
	if len(rc.Y1s) > 0 {
		s += "\n\t"
	}
	s += "}\n"
	s += fmt.Sprintf("\tY2s(%v){", len(rc.Y2s))
	for i, r := range rc.Y2s {
		s += fmt.Sprintf("\n\t\t(%v):%v", i, r)
	}
	if len(rc.Y2s) > 0 {
		s += "\n\t"
	}
	s += "}\n"
	return s
}

func (rc *RingCol) appendToY1(ridx int, label maths.Label, ys []YPart) {
YLoop:
	for i := 0; i < len(ys); i++ {
		for j := range rc.Y1s {
			if rc.Y1s[j].Y == ys[i].Y {
				rc.Y1s[j].Descs = append(rc.Y1s[j].Descs,
					RingDesc{Idx: ridx, PtIdx: ys[i].Idx, Label: label},
				)
				continue YLoop
			}
		}
		// Did not find Y, append it.
		rc.Y1s = append(rc.Y1s, YEdge{
			Y:     ys[i].Y,
			Descs: []RingDesc{{Idx: ridx, PtIdx: ys[i].Idx, Label: label}},
		})

	}
}
func (rc *RingCol) appendToY2(ridx int, label maths.Label, ys []YPart) {
YLoop:
	for i := 0; i < len(ys); i++ {
		for j := range rc.Y2s {
			if rc.Y2s[j].Y == ys[i].Y {
				rc.Y2s[j].Descs = append(rc.Y2s[j].Descs,
					RingDesc{Idx: ridx, PtIdx: ys[i].Idx, Label: label},
				)
				continue YLoop
			}
		}
		// Did not find Y, append it.
		rc.Y2s = append(rc.Y2s, YEdge{
			Y:     ys[i].Y,
			Descs: []RingDesc{{Idx: ridx, PtIdx: ys[i].Idx, Label: label}},
		})

	}
}
func (rc *RingCol) addPts(hm hitmap.Interface, b *Builder, pts1, pts2 []maths.Pt) {
	pts := append(append([]maths.Pt{}, pts1...), pts2...)

	tri := maths.Triangle{pts[0], pts[1], pts[2]}
	label := hm.LabelFor(tri.Center())
	if ring, x1, y1s, x2, y2s, new := b.AddPts(label, pts1, pts2); new {
		// We have a new ring.
		ridx := len(rc.Rings)
		rc.X1, rc.X2 = x1, x2
		rc.Rings = append(rc.Rings, ring)
		rc.appendToY1(ridx, ring.Label, y1s)
		rc.appendToY2(ridx, ring.Label, y2s)
		if !rc.foundInside {
			rc.foundInside = ring.Label == maths.Inside
		}
	}
}

func (rc *RingCol) searchY1(y float64, fn func(idx int, ptIdx int, l maths.Label) bool) {
	if rc == nil {
		return
	}
	for _, yedge := range rc.Y1s {
		if y < yedge.Y {
			return
		}
		if y != yedge.Y {
			continue
		}
		for _, desc := range yedge.Descs {
			if !fn(desc.Idx, desc.PtIdx, desc.Label) {
				return
			}
		}
		return
	}
	return
}
func (rc *RingCol) searchY2(y float64, fn func(idx int, ptIdx int, l maths.Label) bool) {
	if rc == nil {
		return
	}
	for _, yedge := range rc.Y2s {
		if y < yedge.Y {
			return
		}
		if y != yedge.Y {
			continue
		}
		for _, desc := range yedge.Descs {
			if !fn(desc.Idx, desc.PtIdx, desc.Label) {
				return
			}
		}
		return
	}
	return
}

func (rc *RingCol) searchEdge(edge []YEdge, y1, y2 float64, fn func(idx int, ptIdx int, l maths.Label) bool) {

	if rc == nil {
		return
	}
	// Goal: Search through the rings looking for a ring that has a edge based on y1, and y2 and the x of the given YEdge.
	// Work through the edge looking for A Y that matches. Since we now that edge is sorted, we need to first
	// order y1 and y2.
	var wantn bool
	if y1 > y2 {
		y1, y2 = y2, y1
		wantn = true
	}
	switchfn := func(desc RingDesc, nptid int) bool {
		if wantn {
			return fn(desc.Idx, nptid, desc.Label)
		}
		return fn(desc.Idx, desc.PtIdx, desc.Label)
	}
	for i := range edge {
		if y1 < edge[i].Y {
			// We have passed smallest point.
			return
		}
		if y1 != edge[i].Y {
			continue
		}
		//log.Println("Found possible jump point.", edge[i], y1, y2)
		for _, desc := range edge[i].Descs {
			px := rc.Rings[desc.Idx].Points[desc.PtIdx].X
			pptid := desc.PtIdx - 1
			if pptid < 0 {
				pptid = len(rc.Rings[desc.Idx].Points) - 1
			}
			ppt := rc.Rings[desc.Idx].Points[pptid]

			// We need to check is the previous point create the edge?
			if ppt.X == px && ppt.Y == y2 {
				// Okay we have found an edge.
				if !switchfn(desc, pptid) {
					return
				}
			}
			nptid := desc.PtIdx + 1
			if nptid >= len(rc.Rings[desc.Idx].Points) {
				nptid = 0
			}
			npt := rc.Rings[desc.Idx].Points[nptid]
			if npt.X == px && npt.Y == y2 {
				// Okay we have found an edge.
				if !switchfn(desc, nptid) {
					return
				}
			}

		}
	}

}

func (rc *RingCol) searchY1Edge(y1, y2 float64, fn func(idx int, ptIdx int, l maths.Label) bool) {
	rc.searchEdge(rc.Y1s, y1, y2, fn)
}
func (rc *RingCol) searchY2Edge(y1, y2 float64, fn func(idx int, ptIdx int, l maths.Label) bool) {
	rc.searchEdge(rc.Y2s, y1, y2, fn)
}

type mplysByArea struct {
	pmap map[int]int
	ply  [][][]maths.Pt
}

func (mp mplysByArea) Len() int { return len(mp.ply) }
func (mp mplysByArea) Swap(i, j int) {
	li := mp.pmap[i]
	mp.pmap[i] = mp.pmap[j]
	mp.pmap[j] = li
	mp.ply[i], mp.ply[j] = mp.ply[j], mp.ply[i]
}
func (mp mplysByArea) Less(i, j int) bool {
	return points.SinArea(mp.ply[i][0]) < points.SinArea(mp.ply[j][0])
}

func (rc *RingCol) MultiPolygon() [][][]maths.Pt {
	if rc == nil || rc.Rings == nil {
		return nil
	}

	var discardPlys = make([]bool, len(rc.Rings))
	var outsidePlys []int
	var rings [][][]maths.Pt
	var miny, maxy float64

	// used to remove outside rings. If their bounding box touches these then they can be removed.
	if len(rc.Y1s) > 0 {
		miny, maxy = rc.Y1s[0].Y, rc.Y1s[0].Y
	} else if len(rc.Y2s) > 0 {
		miny, maxy = rc.Y2s[0].Y, rc.Y2s[0].Y
	}

	// Mark any polygon touching the left and right border as being able to be discarded.
	// Start with the left border
	for _, yedge := range rc.Y1s {
		if miny > yedge.Y {
			miny = yedge.Y
		}
		if maxy < yedge.Y {
			maxy = yedge.Y
		}
		for _, desc := range yedge.Descs {
			if desc.Label == maths.Outside {
				discardPlys[desc.Idx] = true
				continue
			}
		}

	}

	// Now with the right border.
	for _, yedge := range rc.Y2s {
		if miny > yedge.Y {
			miny = yedge.Y
		}
		if maxy < yedge.Y {
			maxy = yedge.Y
		}
		for _, desc := range yedge.Descs {
			if desc.Label == maths.Outside {
				discardPlys[desc.Idx] = true
				continue
			}
		}
	}

	idxmap := make(map[int]int)
	segmap := make(map[int]hitmap.Segment)

	for i, ring := range rc.Rings {

		// We can discard this ring.
		if discardPlys[i] {
			continue
		}

		if ring.Label == maths.Outside {
			bb := ring.BBox()
			// the ring touches the the top or bottom boader.
			if bb[1] == miny || bb[3] == maxy {
				continue
			}
			// Save for later processing.
			outsidePlys = append(outsidePlys, i)
			continue
		}
		// This is an inside ring. Make a copy.
		idxmap[len(rings)] = i
		lnring := ring.LineRing()
		segmap[len(rings)] = hitmap.NewSegmentFromRing(maths.Inside, ring.Points)
		rings = append(rings, [][]maths.Pt{lnring})
	}
	// we need to sort the rings by area.
	/*
		sort.Sort(mplysByArea{
			pmap: idxmap,
			ply:  rings,
		})
	*/

	// Now run through all the outside Rings.
	for _, i := range outsidePlys {
		obb := points.BoundingBox(rc.Rings[i].BBox())

		for j := len(rings) - 1; j >= 0; j-- {
			ibb := points.BoundingBox(points.BBox(rings[j][0]))
			if ibb.Area() <= obb.Area() {
				continue
			}
			containsbb := ibb.ContainBB(obb)
			if !containsbb {
				continue
			}

			lnring := rc.Rings[i].LineRing()
			//log.Println("Checking to see if the ring contains", lnring[0], "\n", segmap[j], "\n", ibb)
			if !segmap[j].Contains(lnring[0]) {
				continue
			}
			rings[j] = append(rings[j], lnring)
			// Go to the next outside polygon.
			break
			//}
		}
	}
	return rings

}

func BuildRingCol(ctx context.Context, hm hitmap.Interface, col1, col2 []maths.Pt, pt2my map[maths.Pt]int64) (col RingCol) {
	var len1, len2 = len(col1), len(col2)

	var b Builder

	i := 0
	for j := 0; j < len2-1; j++ {
		for i < len1 {
			// Context cancelled.
			if ctx.Err() != nil {
				return col
			}

			maxy, ok := pt2my[col1[i]]
			if (i == len1-1) || (ok && int64(col2[j].Y*100) < maxy) {

				// We can not draw a line to i+1
				// for one of two reasons
				// 1. there is no i+1
				// 2. there is already a line from i to some point beyound j+1 blocking our path.
				col.addPts(hm, &b, col1[i:i+1], col2[j:j+2])
				// move to the next j.
				break
			}

			col.addPts(hm, &b, col1[i:i+2], col2[j:j+1])
			i++
		}
	}
	// Context cancelled.
	if ctx.Err() != nil {
		return col
	}
	for i < len1-1 {
		// Need to fill out the triangles from the last point in j to the last point in i.
		col.addPts(hm, &b, col1[i:i+2], []maths.Pt{col2[len2-1]})
		i++
	}

	// We need to check if there is one last ring in the builder.
	ring, x1, y1s, x2, y2s := b.CurrentRing()
	if len(ring.Points) == 0 {
		// We did not find any rings that were marked as inside
		// We don't care about these rings.
		if !col.foundInside {
			col.Rings = nil
		}
		sort.Sort(EdgeByY(col.Y1s))
		sort.Sort(EdgeByY(col.Y2s))
		return col
	}
	col.X1 = x1
	col.X2 = x2
	ridx := len(col.Rings)
	col.Rings = append(col.Rings, ring)
	col.appendToY1(ridx, ring.Label, y1s)
	col.appendToY2(ridx, ring.Label, y2s)
	if !col.foundInside {
		col.foundInside = ring.Label == maths.Inside
	}
	if !col.foundInside {
		col.Rings = nil
	}
	// Context cancelled.
	if ctx.Err() != nil {
		return col
	}
	sort.Sort(EdgeByY(col.Y1s))
	sort.Sort(EdgeByY(col.Y2s))
	return col
}

func slopeCheck(pt1, pt2, pt3 maths.Pt, x1, x2 float64) bool {
	// if vertical can not do it.
	if (pt1.X == x1 && pt2.X == x2 && pt3.X == x2) ||
		(pt1.X == x1 && pt2.X == x2 && pt3.X == x2) {
		return false
	}
	if pt1.Y == pt2.Y && pt1.Y == pt3.Y {
		return true
	}

	m1, _, d1 := maths.Line{pt1, pt2}.SlopeIntercept()
	m2, _, d2 := maths.Line{pt1, pt3}.SlopeIntercept()
	return d1 && d2 && m1 == m2
}

func merge2AdjectRC(c1, c2 RingCol) (col RingCol) {
	seenRings := make(map[[2]int]bool)
	//var skipNextCol bool
	xc := c1.X2
	cols := [2]RingCol{c1, c2}

	col.X1 = c1.X1
	col.X2 = c2.X2
	var ocoli, ccoli, ptid, nptid int

	var searchCol = func(coli int, y1, y2 float64, fn func(idx int, pidx int, l maths.Label) bool) {
		if coli == 0 {
			cols[0].searchY2Edge(y1, y2, fn)
			return
		}
		cols[1].searchY1Edge(y1, y2, fn)
	}

	var ringsToProcess [][2]int

	// First we are going to loop through y2 of col zero and take notes of the rings.
	for i := range c1.Y2s {
		for _, d := range c1.Y2s[i].Descs {
			if _, ok := seenRings[[2]int{0, d.Idx}]; ok {
				continue
			}
			seenRings[[2]int{0, d.Idx}] = false
			ringsToProcess = append(ringsToProcess, [2]int{0, d.Idx})
		}
	}
	// Go through the rings that tourch the Y1 edge only and add them to our list of rings.
	for i := range c1.Y1s {
		for _, d := range c1.Y1s[i].Descs {
			// Skip any rings that are touching Y2 as well.
			if _, ok := seenRings[[2]int{0, d.Idx}]; ok {
				continue
			}
			seenRings[[2]int{0, d.Idx}] = false
			col.Rings = append(col.Rings, c1.Rings[d.Idx])
		}
	}
	// Add rings that do not touch either edge to our col's rings list.
	for i := range c1.Rings {
		if _, ok := seenRings[[2]int{0, i}]; ok {
			continue
		}
		col.Rings = append(col.Rings, c1.Rings[i])
	}

	// Now we need to do the same thing for col one.

	// Next we are going to loop through y1 of col one and take notes of the rings.
	for i := range c2.Y1s {
		for _, d := range c2.Y1s[i].Descs {
			if _, ok := seenRings[[2]int{1, d.Idx}]; ok {
				continue
			}
			seenRings[[2]int{1, d.Idx}] = false
			ringsToProcess = append(ringsToProcess, [2]int{1, d.Idx})
		}
	}
	// Now we want to go through our Y2s and add those polygons to our polygon list and update col.Y2.
	for i := range c2.Y2s {
		for _, d := range c2.Y2s[i].Descs {
			// Skip any rings that are touching Y2 as well.
			if _, ok := seenRings[[2]int{1, d.Idx}]; ok {
				continue
			}
			seenRings[[2]int{1, d.Idx}] = false
			col.Rings = append(col.Rings, c2.Rings[d.Idx])
		}
	}
	// Now go through the c1 to find and add the rings that don't touch the sides.
	for i := range c2.Rings {
		if _, ok := seenRings[[2]int{1, i}]; ok {
			continue
		}
		col.Rings = append(col.Rings, c2.Rings[i])
	}

	stime := time.Now()

	for p := range ringsToProcess {

		c, r := ringsToProcess[p][0], ringsToProcess[p][1]
		if seenRings[[2]int{c, r}] {
			// It's been processed; skip.
			continue
		}
		seenRings[[2]int{c, r}] = true

		var nring Ring
		nring.Label = cols[c].Rings[r].Label
		ptid = 0
		nptid = 1
		ccoli = c
		if ccoli == 1 {
			ocoli = 0
		} else {
			ocoli = 1
		}
		cri := r
		pt := cols[ccoli].Rings[cri].Points[ptid]
		npt := cols[ccoli].Rings[cri].Points[nptid]
		ptmap := make(map[maths.Pt]int)
		ptcounter := make(map[maths.Pt]int)
		walkedRings := [][2]int{[2]int{c, r}}
		walkedPts := []string{}
		for {
			etime := time.Now()
			elapsed := etime.Sub(stime)
			if elapsed.Minutes() > 10 {
				//if elapsed.Seconds() > 1 {
				fn := genWriteoutCols(c1, c2)
				log.Println("Taking too long, writing file to ", fn)

				panic("Took too long")
			}
			if ptcounter[pt] > 5 {
				log.Println("Walked Pts:", walkedPts)
				log.Println("Col1:", c1.String())
				log.Println("Col2:", c2.String())
				log.Println("On ring:", ccoli, cri)
				log.Println(cols[ccoli].Rings[cri].Points)
				pi := walkedRings[len(walkedRings)-2]
				log.Println("Previous ring:", pi[0], pi[1])
				log.Println(cols[pi[0]].Rings[pi[1]].Points)
				log.Println("Processing ", p, "(", ringsToProcess[p], ") of the following rings that needed to be processed.:", ringsToProcess)
				log.Println(cols[ringsToProcess[p][0]].Rings[ringsToProcess[p][1]].Points)
				log.Println("Walked rings:", walkedRings)
				fn := genWriteoutCols(c1, c2)
				log.Println("Wrote out columns info to:", fn)
				writeOutSVG(fn, cols[:], walkedRings)

				panic("Inif loop?")
			}

			walkedPts = append(walkedPts, fmt.Sprintln(pt))
			if idx, ok := ptmap[pt]; ok {
				// Need to remove the bubble.
				// need to delete the points from the ptmap first.
				for _, pt1 := range nring.Points[idx:] {
					delete(ptmap, pt1)
				}
				nring.Points = nring.Points[:idx]
			}
			if len(nring.Points) > 1 && slopeCheck(nring.Points[len(nring.Points)-2], nring.Points[len(nring.Points)-1], pt, xc, xc) {
				// have the same slope and not vertical
				// can override last point.
				delete(ptmap, nring.Points[len(nring.Points)-1])
				nring.Points[len(nring.Points)-1] = pt
			} else {
				nring.Points = append(nring.Points, pt)
				ptcounter[pt]++
			}
			ptmap[pt] = len(nring.Points) - 1
			if pt.X != xc || npt.X != xc {
				goto NextPoint
			}
			searchCol(ocoli, pt.Y, npt.Y, func(idx int, pidx int, l maths.Label) bool {
				if l != nring.Label {
					return true
				}
				// We have found our canidate. Need to switch over to it.

				ocri := cri
				ptid = pidx
				nptid = ptid + 1
				// swap columns
				ccoli, ocoli = ocoli, ccoli
				cri = idx
				if nptid >= len(cols[ccoli].Rings[cri].Points) {
					nptid = 0
				}
				//log.Println("Marking Ring as seen", ccoli, idx)
				seenRings[[2]int{ccoli, idx}] = true
				walkedRings = append(walkedRings, [2]int{ccoli, idx})
				walkedPts = append(walkedPts,
					fmt.Sprintf("Jumping to Col: %v Ring %v, Pt[%v] %v -- because of %v : %v\n",
						ccoli,
						idx,
						ptid,
						cols[ccoli].Rings[cri].Points[ptid],
						pt, npt,
					),
				)
				cols[ccoli].Rings[cri].BBox()
				// don't continue searching.
				// Let's check the other column real quick with the new edge.
				pt := cols[ccoli].Rings[cri].Points[ptid]
				npt := cols[ccoli].Rings[cri].Points[nptid]
				ptcounter[pt]++
				// This is not an edge.
				if npt.X != pt.X {
					return false
				}
				//log.Println("Searching other col for edge.", ocoli, pt.Y, npt.Y)

				searchCol(ocoli, pt.Y, npt.Y, func(idx int, pidx int, l maths.Label) bool {
					if l != nring.Label {
						return true
					}
					// Don't want this polygon.
					if idx == ocri {
						return true
					}
					// We have found our canidate. Need to switch over to it.
					//	log.Println("Found edge (", pt, "-", npt, ") in our col", ocoli, idx, pidx)
					ptid = pidx
					nptid = ptid + 1
					// swap columns
					ccoli, ocoli = ocoli, ccoli
					cri = idx
					//log.Println("Marking Ring as seen", ccoli, idx)
					seenRings[[2]int{ccoli, idx}] = true
					walkedRings = append(walkedRings, [2]int{ccoli, idx})
					walkedPts = append(walkedPts,
						fmt.Sprintf("Jumping to Col: %v Ring %v, Pt[%v] %v -- because of %v : %v\n",
							ccoli,
							idx,
							ptid,
							cols[ccoli].Rings[cri].Points[ptid],
							pt, npt,
						),
					)
					cols[ccoli].Rings[cri].BBox()
					if nptid >= len(cols[ccoli].Rings[cri].Points) {
						nptid = 0
					}
					return false

				})
				return false
			})

		NextPoint:
			// Move to the next point
			ptid, nptid = nptid, nptid+1
			if nptid >= len(cols[ccoli].Rings[cri].Points) {
				nptid = 0
			}
			pt = cols[ccoli].Rings[cri].Points[ptid]
			npt = cols[ccoli].Rings[cri].Points[nptid]
			if pt.IsEqual(nring.Points[0]) {
				break
			}
		}
		plen := len(nring.Points)
		if plen > 3 {
			switch {
			// Let's check the second to last pt, last pt, and the first pt to see if the last point can be dropped.
			case slopeCheck(nring.Points[plen-2], nring.Points[plen-1], nring.Points[0], col.X1, col.X2):
				nring.Points = nring.Points[:plen-1]
				// Let's check the  last pt, and the first two pts to see if the first point can be dropped.
			case slopeCheck(nring.Points[plen-1], nring.Points[0], nring.Points[1], col.X1, col.X2):
				nring.Points = nring.Points[1:]
			}
		}
		if plen < 3 {
			fn := genWriteoutCols(c1, c2)
			log.Println("Generated a ring with fewer then 3 points: ", fn, nring)

			panic("Generated a ring with fewer then 3 points. ")
		}
		points.RotateToLowestsFirst(nring.Points)

		col.Rings = append(col.Rings, nring)

	}
	// Calculate out our indexs.
	for i, r := range col.Rings {
		for j, pt := range r.Points {
			switch pt.X {
			case col.X1:
				col.appendToY1(i, r.Label, []YPart{{Y: pt.Y, Idx: j}})
			case col.X2:
				col.appendToY2(i, r.Label, []YPart{{Y: pt.Y, Idx: j}})
			}
		}
	}
	// Need to sort the Y's from top to bottom.
	sort.Sort(EdgeByY(col.Y1s))
	sort.Sort(EdgeByY(col.Y2s))

	// Verify that the cols indexes are pointed correctly.
	for i := range col.Y1s {
		cpt := maths.Pt{col.X1, col.Y1s[i].Y}
		for j, d := range col.Y1s[i].Descs {
			ring := col.Rings[d.Idx]
			if d.Label != ring.Label {
				col.Y1s[i].Descs[j].Label = ring.Label
			}

			pt := ring.Points[d.PtIdx]
			if !cpt.IsEqual(pt) {
				// loop through the ring, and find the correct id.
				var found bool
				for r := range ring.Points {
					if cpt.IsEqual(ring.Points[r]) {
						col.Y1s[i].Descs[j].PtIdx = r
						found = true
						break
					}
				}
				if !found {
					log.Println("col", col.String())
					log.Println("Did not find r when trying to fix up Y1.", i, j)
					panic("Did not find r when trying to fix up Y1.")
				}
			}

		}

	}
	for i := range col.Y2s {
		cpt := maths.Pt{col.X2, col.Y2s[i].Y}
		for j, d := range col.Y2s[i].Descs {
			ring := col.Rings[d.Idx]
			if d.Label != ring.Label {
				col.Y2s[i].Descs[j].Label = ring.Label
			}
			pt := ring.Points[d.PtIdx]
			if !cpt.IsEqual(pt) {
				// loop through the ring, and find the correct id.
				var found bool
				for r := range ring.Points {
					if cpt.IsEqual(ring.Points[r]) {
						col.Y2s[i].Descs[j].PtIdx = r
						found = true
						break
					}
				}
				if !found {
					log.Println("col", col.String())
					log.Println("Did not find r when trying to fix up Y2.", i, j)
					panic("Did not find r when trying to fix up Y2.")
				}
			}
		}
	}
	return col
}

func MergeCols(cols []RingCol) RingCol {
	lcol := cols[0]
	for i := 1; i < len(cols); i++ {
		lcol = merge2AdjectRC(lcol, cols[i])
	}
	return lcol
}

func GenerateMultiPolygon(cols []RingCol) (plys [][][]maths.Pt) {
	var lock sync.Mutex
	var wg sync.WaitGroup
	var wChan = make(chan [2]int)
	var numWorkers = runtime.NumCPU()

	li := -1
	var worker = func(id int) {
		for i := range wChan {
			wcol := MergeCols(cols[i[0]:i[1]])
			wply := wcol.MultiPolygon()
			lock.Lock()
			for i := range wply {
				plys = append(plys, wply[i])
			}
			lock.Unlock()
		}
		wg.Done()
	}
	for i := 0; i < numWorkers; i++ {
		go worker(i)
	}
	wg.Add(numWorkers)

	for i := range cols {
		if len(cols[i].Rings) == 0 {
			if li != -1 {
				// We need to do some work.
				wChan <- [2]int{li, i}
				li = -1
			}
			continue
		}
		if li == -1 {
			li = i
		}
	}
	if li != -1 {
		wChan <- [2]int{li, len(cols)}
	}
	close(wChan)
	wg.Wait()
	return plys
}

func writeOutSVG(fn string, cols []RingCol, onlyRings [][2]int) {
	var filter bool
	ringFilter := make(map[[2]int]bool)
	if len(onlyRings) > 0 {
		filter = true
		for i := range onlyRings {
			ringFilter[onlyRings[i]] = true
		}
	}
	f, err := os.Create(fn + ".svg")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	canvas := svg.New(f)
	canvas.Startview(786, 1024, int(cols[0].X1)-10, 2000, int(cols[1].X2)+10, 2200)
	defer canvas.End()

	canvas.Def()
	canvas.Marker("markerCircle", 1, 1, 0, 0)
	canvas.Circle(5, 5, 1, "stroke:none;fill:#8a8a8a;fill-opacity:0.3")
	canvas.MarkerEnd()
	canvas.DefEnd()

	style := func(l maths.Label, i int) string {
		if i == 0 {

			if l == maths.Inside {
				return "fill:#0000ff;fill-opacity:0.3;stroke:none;marker-mid: url(#markerCircle)"
			}
			return "fill:#ff0000;fill-opacity:0.3;stroke:none; marker-mid: url(#markerCircle)"
		}
		if l == maths.Inside {
			return "fill:#0088ff;fill-opacity:0.3;stroke:none; marker-mid: url(#markerCircle)"
		}
		return "fill:#ff8800;fill-opacity:0.3;stroke:none; marker-mid: url(#markerCircle)"

	}

	pointsToIntArray := func(pts []maths.Pt) (xs []int, ys []int) {
		for _, pt := range pts {
			xs = append(xs, int(pt.X))
			ys = append(ys, int(pt.Y))
		}
		return xs, ys
	}
	pointmap := make(map[maths.Pt]struct{})

	for i, col := range cols {
		for j, r := range col.Rings {
			if filter && ringFilter[[2]int{i, j}] {
				continue
			}
			for _, pt := range r.Points {
				pointmap[pt] = struct{}{}
			}
		}
	}

	canvas.Scale(1.5)
	// Draw the X1,X2, X2 lines
	canvas.Line(int(cols[0].X1), -20, int(cols[0].X1), 4126, "stroke:#8a8a8a")
	canvas.Line(int(cols[0].X2), -20, int(cols[0].X2), 4126, "stroke:#8a8a8a")
	canvas.Line(int(cols[1].X2), -20, int(cols[1].X2), 4126, "stroke:#8a8a8a")

	for i, c := range cols {
		for j, r := range c.Rings {
			if filter && ringFilter[[2]int{i, j}] {
				continue
			}
			xs, ys := pointsToIntArray(r.Points)
			canvas.Polygon(xs, ys, style(r.Label, i))
		}
	}
	canvas.Gend()
}
