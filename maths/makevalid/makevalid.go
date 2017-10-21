// +build !xsweep

package makevalid

import (
	"sort"

	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap"
)

// makeValid takes a set of polygons that is invalid,
// will include triangles outside of the polygons provided, creating a convex hull.
func makeValid(plygs ...[]maths.Line) (polygons [][][]maths.Pt, err error) {
	//defer trace(fmt.Sprintf("makeValid(%v --\n%#v\n): ", len(plygs), plygs))()

	//stime := time.Now()
	destructuredLines := edgemap.Destructure(edgemap.InsureConnected(plygs...))
	/*
		etime := time.Now()
		log.Println("dstructedLines took: ", etime.Sub(stime))
		stime = etime
	*/
	edgeMap := edgemap.New(destructuredLines)
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
		panic(err)
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
