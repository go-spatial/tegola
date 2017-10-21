// +build xsweep

package makevalid

import (
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap"
	"github.com/terranodo/tegola/maths/hitmap"
)

// makeValid takes a set of polygons that is invalid,
// will include triangles outside of the polygons provided, creating a convex hull.
func makeValid(hm hitmap.Interface, extent float64, plygs ...[]maths.Line) (polygons [][][]maths.Pt, err error) {

	//defer gotrace.Trace(fmt.Sprintf("makevalid(%v):", len(plygs)))()
	//defer trace(fmt.Sprintf("makeValid(%v --\n%#v\n): ", len(plygs), plygs))()

	//triangleGraph := edgemap.GenerateTriangleGraph(hm, adjustBBoxBy, edgemap.InsureConnected(plygs...))

	triangleGraph, _ := edgemap.GenerateTriangleGraph1(
		hm,
		0.0,
		edgemap.InsureConnected(plygs...),
		extent,
	)

	polygons = triangleGraph.TrianglesAsMP()
	/*
		//	fn := gotrace.Trace("Construct rings")
		rings := triangleGraph.Rings()
		for i, r := range rings {
			log.Printf("Got the following rings[%v]: %v", i, len(r))
		}
		for _, ring := range rings {
			polygon := constructPolygon(ring)
			polygons = append(polygons, polygon)
		}
		//	fn()
		/*
			etime = time.Now()
			log.Println("Rings and ConstructPolygon took: ", etime.Sub(stime))
	*/
	// Need to sort the polygons in the multipolygon to get a consistent order.
	//sort.Sort(plygByFirstPt(polygons))
	return polygons, nil

}
