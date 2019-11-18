package subdivision

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/gdey/errors"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/internal/debugger"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/quadedge"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/subdivision/pseudopolygon"
	"github.com/go-spatial/geom/winding"
)

func roundGeomPoint(pt geom.Point) geom.Point {
	return geom.Point{
		math.Round(pt[0]*RoundingFactor) / RoundingFactor,
		math.Round(pt[1]*RoundingFactor) / RoundingFactor,
	}

}

func ResolveStartingEndingEdges(vertexIndex VertexIndex, start, end geom.Point) (startingEdge, endingEdge *quadedge.Edge, exists bool, err error) {
	var (
		ok         bool
		eerr, serr error
	)

	start = roundGeomPoint(start)
	end = roundGeomPoint(end)

	startingEdge, ok = vertexIndex.Get(start)
	if !ok {
		// start is not in our subdivision
		return nil, nil, false, ErrInvalidStartingVertex
	}

	if e := startingEdge.FindONextDest(end); e != nil {
		// Nothing to do, edge already in the subdivision.
		return nil, nil, true, nil
	}

	endingEdge, ok = vertexIndex.Get(end)
	if !ok {
		// start is not in our subdivision
		return nil, nil, false, ErrInvalidEndVertex
	}

	startingEdge, serr = quadedge.ResolveEdge(startingEdge, end)
	endingEdge, eerr = quadedge.ResolveEdge(endingEdge, start)

	if debug {
		log.Printf("startingEdge: %v, err: %v", wkt.MustEncode(startingEdge.AsLine()), serr)
		log.Printf("endingEdge: %v, err: %v", wkt.MustEncode(endingEdge.AsLine()), eerr)
	}
	if serr == geom.ErrPointsAreCoLinear && eerr == geom.ErrPointsAreCoLinear {
		// the starting edge and ending edge end at the same place.
		// the request edge overlaps a set of existing edges.
		return nil, nil, true, nil
	}
	return startingEdge, endingEdge, false, nil
}

func (sd *Subdivision) InsertConstraint(ctx context.Context, vertexIndex VertexIndex, start, end geom.Point) (err error) {

	if cgo && debug {

		ctx = debugger.AugmentContext(ctx, "")
		defer debugger.Close(ctx)

		defer func() {
			if err != nil && err != ErrCoincidentalEdges {
				log.Printf("starting point %#v\n", start)
				log.Printf("end point %#v\n", end)
				log.Printf("err: %v", err)
			}
		}()
	}

	if vertexIndex == nil {
		vertexIndex = sd.VertexIndex()
	}

	startingEdge, endingEdge, exist, err := ResolveStartingEndingEdges(vertexIndex, start, end)
	if err != nil {
		var dumpStr strings.Builder

		DumpSubdivisionW(&dumpStr, sd)
		log.Print(dumpStr.String())
		return err

	}
	if exist {
		return nil
	}

	removalList, err := FindIntersectingEdges(startingEdge, endingEdge)
	if err != nil {

		if debug {
			var dumpStr strings.Builder

			fmt.Fprintf(&dumpStr, "starting edge: %v\n", wkt.MustEncode(startingEdge.AsLine()))
			fmt.Fprintf(&dumpStr, "ending edge: %v\n", wkt.MustEncode(endingEdge.AsLine()))
			fmt.Fprintf(&dumpStr, "intersecting edge: %v\n", wkt.MustEncode(
				geom.LineString{
					[2]float64(*startingEdge.Orig()),
					[2]float64(end),
				},
			))
			DumpSubdivisionW(&dumpStr, sd)
			fmt.Fprintf(&dumpStr, `
testcase:
{
	Lines: must.ReadMultilines($filename),
	Start: %#v,
	End: %#v,
},`, *startingEdge.Orig(), end)
			log.Print(dumpStr.String())
		}

		return err

	}
	if len(removalList) == 0 {
		// nothing to do.
		return nil
	}

	if debug {
		log.Printf("got %v edges to remove", len(removalList))
	}

	pppc := PseudoPolygonPointCollector{
		Start: start,
		End:   end,
		Order: sd.Order,
	}

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

		if err = pppc.AddEdge(e); err != nil {
			if debug {
				log.Printf("Failed to added edge (%v) to pppc", wkt.MustEncode(e.AsLine()))
				log.Printf("StartingEdge: %v , end: %v", startingEdge.AsLine(), end)
			}
			return err
		}

		vertexIndex.Remove(e)
		quadedge.Delete(e)

	}

	if debug {
		pppc.debugRecord(ctx)
	}

	// let's do the lines that are counter clock-wise first
	for _, lbl := range []string{"lower", "upper"} {

		edges, err := pppc.Edges(lbl == "upper")
		if err != nil {
			if debug {
				log.Println("triangulate pseudo polygon fail.", err)
			}
			return err
		}

		if debug {
			for i, edg := range edges {
				debugger.Record(ctx, edg, "pseudo polygon edge", "%v line %v : %v", lbl, i, edg)
			}
		}

		var redoedges []int
		for i, edge := range edges {

			if debug {
				log.Printf("attempting to add edge %05v of %05v", i, len(edges))
			}
			if err = sd.insertEdge(vertexIndex, edge[0], edge[1]); err != nil {
				if err == ErrDidNotFindToFrom {
					log.Printf("Failed to find edge: %v", wkt.MustEncode(edge))
					// let's requeue this edge
					redoedges = append(redoedges, i)
					continue
				}
				if debug {
					log.Printf("Failed to insert edge: %v -- %v.", err, wkt.MustEncode(edge))
				}
				return err
			}
		}
		for _, i := range redoedges {
			if err = sd.insertEdge(vertexIndex, edges[i][0], edges[i][1]); err != nil {
				log.Println("Redo Failed to insert edge.", len(redoedges))
			}
		}
	}

	return nil
}

func (sd *Subdivision) insertEdge(vertexIndex VertexIndex, start, end geom.Point) error {
	if vertexIndex == nil {
		vertexIndex = sd.VertexIndex()
	}

	start, end = roundGeomPoint(start), roundGeomPoint(end)

	tempEdge, ok := vertexIndex.Get(start)
	if !ok {
		// start is not in our subdivision
		return ErrInvalidStartingVertex
	}
	if tempEdge.FindONextDest(end) != nil {
		// edge already exists do nothing.
		if debug {
			log.Printf("found edge in the system, not adding (%v,%v)", start, end)
		}
		return nil
	}

	from, err := quadedge.ResolveEdge(tempEdge, end)
	// There are two errors we care about
	switch err {
	case nil:
		// do nothing
	case geom.ErrPointsAreCoLinear:
		// this edge already exists.
		return nil
	default:
		return err
	}

	if from == nil {
		if debug {
			DumpSubdivision(sd)
			log.Printf("end:\n %v\n tempEdge :\n%v\n", wkt.MustEncode(end), wkt.MustEncode(tempEdge.AsLine()))
			log.Printf("%v", err)
			log.Printf("tempedge:\n%v", tempEdge.DumpAllEdges())
		}
		return ErrInvalidStartingVertex
	}

	tempEdge, ok = vertexIndex.Get(end)
	if !ok {
		// end is not in our subdivision
		return ErrInvalidEndVertex
	}

	to, err := quadedge.ResolveEdge(tempEdge, start)
	switch err {
	case nil:
		// do nothing
	case geom.ErrPointsAreCoLinear:
		// this edge already exists.
		return nil
	default:
		return err
	}
	if to == nil {
		if debug {
			DumpSubdivision(sd)
			log.Printf("start:\n %v\n tempEdge :\n%v\n", wkt.MustEncode(start), wkt.MustEncode(tempEdge.AsLine()))
			log.Printf("%v", err)
			log.Printf("tempedge:\n%v", tempEdge.DumpAllEdges())
		}
		return ErrInvalidEndVertex
	}

	newEdge := quadedge.Connect(from.ONext().Sym(), to, sd.Order)

	if debug {
		log.Printf("Connected : %v -> %v", from.ONext().Sym().AsLine(), to.AsLine())
		log.Printf("Added edge %p: %v", newEdge, newEdge.AsLine())
	}

	vertexIndex.Add(newEdge)
	return nil
}

type PseudoPolygonPointCollector struct {
	upperPoints []geom.Point
	lowerPoints []geom.Point
	seen        map[geom.Point]bool
	Start       geom.Point
	End         geom.Point
	Order       winding.Order
}

// AddEdge will attempt to add the origin and dest points of the edge to the lower
// or upper set as required
func (pppc *PseudoPolygonPointCollector) AddEdge(e *quadedge.Edge) error {
	if e == nil {
		return errors.String("edge is nil")
	}

	if err := pppc.AddPoint(*e.Orig()); err != nil {
		return err
	}
	return pppc.AddPoint(*e.Dest())

}

// AddPoint will add the given point to the lower or upper set as required and if
// it's not a point that has already been seen
func (pppc *PseudoPolygonPointCollector) AddPoint(pt geom.Point) error {
	if pppc.seen == nil {
		pppc.seen = make(map[geom.Point]bool)
	}

	if len(pppc.upperPoints) == 0 {
		pppc.upperPoints = append(pppc.upperPoints, pppc.Start)
		pppc.seen[pppc.Start] = true
	}

	if len(pppc.lowerPoints) == 0 {
		pppc.lowerPoints = append(pppc.lowerPoints, pppc.Start)
		pppc.seen[pppc.Start] = true
	}

	if pppc.seen[pt] {
		return nil
	}

	c := quadedge.Classify(pt, pppc.Start, pppc.End)
	switch c {
	case quadedge.LEFT:
		pppc.lowerPoints = append(pppc.lowerPoints, pt)
	case quadedge.RIGHT:
		pppc.upperPoints = append(pppc.upperPoints, pt)
	case quadedge.ORIGIN, quadedge.BETWEEN, quadedge.DESTINATION:
		// a constraint that is colinear

	default:
		if debug {
			log.Printf("Classification: %v -- %v, %v, %v", c, pt, pppc.Start, pppc.End)
			// should not come here.
			return ErrAssumptionFailed()
		}
	}
	return nil

}

// SharedLine returns the line shared by the set of points, all points should be on one side or the other of this line
func (pppc *PseudoPolygonPointCollector) SharedLine() geom.Line {
	return geom.Line{
		[2]float64(pppc.Start),
		[2]float64(pppc.End),
	}
}

// Edges returns the triangulated edges for the upper or lower region
func (pppc *PseudoPolygonPointCollector) Edges(upper bool) ([]geom.Line, error) {
	var pts []geom.Point

	if upper {
		pts = make([]geom.Point, len(pppc.upperPoints))
		copy(pts, pppc.upperPoints)
	} else {
		pts = make([]geom.Point, len(pppc.lowerPoints))
		copy(pts, pppc.lowerPoints)
	}
	if debug {
		lbl := "lower"
		if upper {
			lbl = "upper"

		}
		log.Printf("Working on %v points: %v", lbl, wkt.MustEncode(pts))
	}

	if !pppc.seen[pppc.End] {
		pts = append(pts, pppc.End)
	}

	if len(pts) == 2 {
		// just a shared line, no points to triangulate.
		return []geom.Line{pppc.SharedLine()}, nil
	}

	return pseudopolygon.Triangulate(pts, pppc.Order)
}

func (pppc *PseudoPolygonPointCollector) debugRecord(ctx context.Context) {
	if debug {
		for i, upt := range pppc.upperPoints {
			debugger.Record(ctx, upt, "pseudo polygon", "upper point %v : %v", i, upt)
		}
		for i, lpt := range pppc.lowerPoints {
			debugger.Record(ctx, lpt, "pseudo polygon", "lower point %v : %v", i, lpt)
		}
	}

}
