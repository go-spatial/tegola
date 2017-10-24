package makevalid

import (
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap"
	"github.com/terranodo/tegola/maths/hitmap"
)

// makeValid takes a set of polygons that is invalid,
// will include triangles outside of the polygons provided, creating a convex hull.
func makeValid(hm hitmap.Interface, extent float64, plygs ...[]maths.Line) (polygons [][][]maths.Pt, err error) {

	triangleGraph, _ := edgemap.GenerateTriangleGraph1(
		hm,
		0.0,
		edgemap.InsureConnected(plygs...),
		extent,
	)
	polygons = triangleGraph.TrianglesAsMP()
	return polygons, nil

}
