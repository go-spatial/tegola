package subdivision

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/internal/debugger"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/planar/triangulate/gdey/quadedge/quadedge"
)

func (sd *Subdivision) InsertConstraint(ctx context.Context, vertexIndex VertexIndex, start, end geom.Point) (err error) {

	if cgo && debug {

		ctx = debugger.AugmentContext(ctx, "")
		defer debugger.Close(ctx)

	}
	defer func() {
		if err != nil && err != ErrCoincidentalEdges {
			//DumpSubdivision(sd)
			fmt.Printf("starting point %#v\n", start)
			fmt.Printf("end point %#v\n", end)
		}
	}()

	var (
		pu []geom.Point
		pl []geom.Point
	)

	if vertexIndex == nil {
		vertexIndex = sd.VertexIndex()
	}

	startingEdge, ok := vertexIndex[start]
	if !ok {
		// start is not in our subdivision
		return ErrInvalidStartingVertex
	}

	if e := startingEdge.FindONextDest(end); e != nil {
		// Nothing to do, edge already in the subdivision.
		return nil
	}

	endingEdge, ok := vertexIndex[end]
	if !ok {
		// start is not in our subdivision
		return ErrInvalidEndVertex
	}

	startingEdge = resolveEdge(startingEdge, end)
	endingEdge = resolveEdge(endingEdge, start)

	removalList, err := FindIntersectingEdges(startingEdge, endingEdge)
	if err != nil {
		var (
			id int64
		)
		tdb, err1 := OpenTestDB("subdivision.gpkg")
		if err1 != nil {
			log.Println("err opening test db:", err1)
			goto PANIC
		}
		id, err1 = tdb.WriteContained(
			"FineIntersectingEdge:inifite",
			fmt.Sprintf("err: %v time: %v", err, time.Now()),
			sd,
			start,
			end,
		)
		if err1 != nil {
			log.Println("err opening test db:", err1)
		} else {
			log.Println("id for entry: ", id)
		}
		if err1 = tdb.WriteEdge(
			id,
			"FindintersectingEdges",
			"starting edge",
			startingEdge,
		); err1 != nil {
			log.Printf("err writing %v", err1)
		}
		if err1 = tdb.WriteEdge(
			id,
			"FindintersectingEdges",
			"starting edge ONext",
			startingEdge.ONext(),
		); err1 != nil {
			log.Printf("err writing %v", err1)
		}
		if err1 = tdb.WriteEdge(
			id,
			"FindintersectingEdges",
			"starting edge OPrev",
			startingEdge.OPrev(),
		); err1 != nil {
			log.Printf("err writing %v", err1)
		}
		if err1 = tdb.WriteLineString(
			id,
			"FindintersectingEdges",
			"intersecting edge",
			geom.LineString{
				[2]float64(*startingEdge.Orig()),
				[2]float64(end),
			},
		); err1 != nil {
			log.Printf("err writing %v", err1)
		}
		log.Printf("intersecting edge: %v", wkt.MustEncode(
			geom.LineString{
				[2]float64(*startingEdge.Orig()),
				[2]float64(end),
			},
		))
		if err1 = tdb.WritePoint(
			id,
			"FindintersectingEdges",
			"start point for intersecting edge",
			*startingEdge.Orig(),
		); err1 != nil {
			log.Printf("err writing %v", err1)
		}
		if err1 = tdb.WritePoint(
			id,
			"FindintersectingEdges",
			"end point for intersecting edge",
			end,
		); err1 != nil {
			log.Printf("err writing %v", err1)
		}

		tdb.Close()

	PANIC:
		return err
	}
	log.Printf("got %v edges to remove", len(removalList))

	pu = append(pu, start)
	pl = append(pl, start)

	for i, e := range removalList {
		if IsHardFrameEdge(sd.frame, e) {
			if debug {
				debugger.Record(ctx, e.AsLine(), "edge tagged for removal", "hard edge %v : %v", i, e.AsLine())
			}
			continue
		}
		if debug {
			debugger.Record(ctx, e.AsLine(), "edge tagged for removal", "edge %v : %v", i, e.AsLine())
		}
		for _, spoint := range [2]geom.Point{*e.Orig(), *e.Dest()} {
			c := quadedge.Classify(spoint, start, end)
			log.Printf("Classification: %v -- %v, %v, %v", c, spoint, start, end)
			switch c {
			case quadedge.LEFT:
				pl = appendNonrepeat(pl, spoint)
			case quadedge.RIGHT:
				pu = appendNonrepeat(pu, spoint)
			case quadedge.ORIGIN, quadedge.BETWEEN:
				// a constraint that is coliner
				continue

			default:
				if debug {
					log.Printf("Requested to remove: %v", wkt.MustEncode(e.AsLine()))
					log.Printf("StartingEdge: %v , end: %v", startingEdge.AsLine(), end)
					log.Printf("Classification: %v -- %v, %v, %v", c, spoint, start, end)
					// should not come here.
					return ErrAssumptionFailed()
				}
				continue
			}
		}
		vertexIndex.Remove(e)
		quadedge.Delete(e)
	}

	pl = appendNonrepeat(pl, end)
	pu = appendNonrepeat(pu, end)

	if debug {
		for i, upt := range pu {
			debugger.Record(ctx, upt, "pseudo polygon", "upper point %v : %v", i, upt)
		}
		for i, lpt := range pl {
			debugger.Record(ctx, lpt, "pseudo polygon", "lower point %v : %v", i, lpt)
		}
	}
	for ptType, pts := range [2][]geom.Point{pu, pl} {
		if len(pts) == 2 {
			// just a shared line, no points to triangulate.
			continue
		}

		edges, err := triangulatePseudoPolygon(pts)
		if err != nil {
			if debug {
				log.Println("triangulate pseudo polygon fail.", err)
			}
			return err
		}
		if debug {
			lbl := "upper"
			if ptType == 1 {
				lbl = "lower"

			}
			for i, edg := range edges {
				debugger.Record(ctx, edg, "pseudo polygon edge", "%v line %v : %v", lbl, i, edg)
			}

		}

		var redoedges []int
		for i, edge := range edges {

			// First we need to check that the edge does not intersect other edges, this can happen if
			// the polygon we are  triangulating happens to be concave. In which case it is possible
			// a triangle outside of the "ok" region, and we should ignore those edges

			// Original code think this is a bug: intersectList, _ := intersectingEdges(startingEdge,end)
			{
				/*
					startingEdge := vertexIndex[edge[0]]

					//intersectList, err := IntersectingEdges(ctx, startingEdge, edge[1])
					intersectList, err := IntersectingEdges(ctx, startingEdge, end)
					if err != nil && err != ErrCoincidentEdges {
						log.Println("failed to insert edge check")
						return err
					}
					// filter out intersects only at the end points.
					count := 0
					for _, iln := range intersectList {
						if cmp.GeomPointEqual(*iln.Orig(), edge[0]) ||
							cmp.GeomPointEqual(*iln.Dest(), edge[0]) ||
							cmp.GeomPointEqual(*iln.Orig(), edge[1]) ||
							cmp.GeomPointEqual(*iln.Dest(), edge[1]) {
							continue
						}
						count++
					}
					if count > 0 {
						if debug {
							debugger.Record(ctx,
								edge[0],
								"intersecting line:startPoint",
								"Start Point",
							)
							debugger.Record(ctx,
								edge[1],
								"intersecting line:endPoint",
								"End Point",
							)
							debugger.Record(ctx,
								startingEdge.AsLine(),
								"intersecting line:startingedge",
								"StartingEdge %v", startingEdge.AsLine(),
							)

							l := geom.Line{[2]float64(*startingEdge.Orig()), [2]float64(edge[1])}
							debugger.Record(ctx,
								l,
								"intersecting line:intersecting",
								"should not fine any intersects with this line. %v ", l,
							)
							for i, il := range intersectList {
								debugger.Record(ctx,
									il.AsLine(),
									"intersecting line:intersected",
									"line %v of %v -- %v", i, len(intersectList), il.AsLine(),
								)
							}

						}
						log.Println("number of intersectlist found", count)
						//return errors.New("Should not get here.")
						continue
					}
				*/
			}

			if err = sd.insertEdge(vertexIndex, edge[0], edge[1]); err != nil {
				if err == ErrDidNotFindToFrom {
					// let's requeue this edge
					redoedges = append(redoedges, i)
					continue
				}
				log.Println("Failed to insert edge.")
				return err
			}
		}
		for _, i := range redoedges {
			if err = sd.insertEdge(vertexIndex, edges[i][0], edges[i][1]); err != nil {
				log.Println("Redo Failed to insert edge.", len(redoedges))

				//ignore
				//	return err
			}
		}
	}

	return nil
}

func (sd *Subdivision) insertEdge(vertexIndex VertexIndex, start, end geom.Point) error {
	if vertexIndex == nil {
		vertexIndex = sd.VertexIndex()
	}
	log.Printf("asked to add edge to the system: (%v,%v)", start, end)
	startingedge, ok := vertexIndex[start]
	if !ok {
		// start is not in our subdivision
		return ErrInvalidStartingVertex
	}
	if startingedge.FindONextDest(end) != nil {
		// edge already exists do nothing.
		if debug {
			log.Printf("found edge in the system, not adding (%v,%v)", start, end)
		}
		return nil
	}

	from := resolveEdge(startingedge, end)
	log.Printf("found from edge: %v", from.AsLine())

	startingedge, ok = vertexIndex[end]
	if !ok {
		// end is not in our subdivision
		return ErrInvalidEndVertex
	}

	to := resolveEdge(startingedge, start)

	/*
		log.Println("Looking for to and from.")
		// Now let's find the edge that would be ccw to end
		from, to := findImmediateRightOfEdges(startingedge.ONext(), end)
		log.Printf("found for to and from? %p, %p", from, to)
		if from == nil {
			// The nodes are too far away or the line we are trying to
			// insert crosses and already existing line
			return ErrDidNotFindToFrom
		}
		if to == nil {
			// already in the system
			return nil
		}


			ct, err := FindIntersectingTriangle(edge, end)
			if err != nil && err != ErrCoincidentEdges {
				return err
			}
			if ct == nil {
				return errors.New("did not find an intersecting triangle. assumptions broken.")
			}

			from := ct.StartingEdge().Sym()

			symEdge, ok := vertexIndex[end]
			if !ok || symEdge == nil {
				return errors.New("Invalid ending vertex.")
			}

			ct, err = FindIntersectingTriangle(symEdge, start)
			if err != nil && err != ErrCoincidentEdges {
				return err
			}
			if ct == nil {
				return errors.New("sym did not find an intersecting triangle. assumptions broken.")
			}

			to := ct.StartingEdge().OPrev()
	*/
	newEdge := quadedge.Connect(from.Sym(), to)
	if debug {

		log.Printf("Connected : %v -> %v", from.Sym().AsLine(), to.AsLine())
		log.Printf("Added edge %p: %v", newEdge, newEdge.AsLine())
	}
	vertexIndex.Add(newEdge)
	return nil
}

func IntersectingEdges(ctx context.Context, startingEdge *quadedge.Edge, end geom.Point) (intersected []*quadedge.Edge, err error) {

	if cgo && debug {

		ctx = debugger.AugmentContext(ctx, "")
		defer debugger.Close(ctx)

	}

	var (
		start        = startingEdge.Orig()
		tseq         *Triangle
		pseq         geom.Point
		shared       *quadedge.Edge
		currentPoint = start
	)

	line := geom.Line{[2]float64(*start), [2]float64(end)}

	t, err := FindIntersectingTriangle(startingEdge, end)
	if err != nil {
		return nil, err
	}
	if debug {
		log.Println("First Triangle: ", t.AsGeom())
		debugger.Record(ctx,
			t.AsGeom(),
			"FindIntersectingEdges:Triangle:0",
			"First triangle.",
		)
	}

	for !t.IntersectsPoint(end) {
		if tseq, err = t.OppositeTriangle(*currentPoint); err != nil {
			if debug {
				debugger.Record(ctx,
					tseq.AsGeom(),
					"FindIntersectingEdges:Triangle:Opposite",
					"Opposite triangle.",
				)
			}
			return nil, err
		}
		if debug {
			debugger.Record(ctx,
				tseq.AsGeom(),
				"FindIntersectingEdges:Triangle:Opposite",
				"Opposite triangle.",
			)
		}
		shared = t.SharedEdge(*tseq)
		if shared == nil {
			log.Printf("t: %v", wkt.MustEncode(t.AsGeom()))
			log.Printf("tseq: %v", wkt.MustEncode(tseq.AsGeom()))
			// Should I panic? This is weird.
			return nil, errors.New("did not find shared edge with Opposite Triangle.")
		}
		pseq = *tseq.OppositeVertex(*t)
		switch quadedge.Classify(pseq, *start, end) {
		case quadedge.LEFT:
			currentPoint = shared.Orig()
		case quadedge.RIGHT:
			currentPoint = shared.Dest()
		}
		if _, ok := planar.SegmentIntersect(line, shared.AsLine()); ok {
			intersected = append(intersected, shared)
		}
		t = tseq
	}
	return intersected, nil

}
