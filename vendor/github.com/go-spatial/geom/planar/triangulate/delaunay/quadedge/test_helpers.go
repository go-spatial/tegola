package quadedge

import "github.com/go-spatial/geom"

// BuildEdgeGraphAroundPoint will build an edge and it's surounding point
// as the points are listed. Points should be listed in counter-clockwise
// order for it to build a valid edge graph
func BuildEdgeGraphAroundPoint(ocoord geom.Point, dcoords ...geom.Point) *Edge {
	if len(dcoords) == 0 {
		panic("dccords does not have any points")
	}
	edges := make([]*Edge, len(dcoords))
	for i := range dcoords {
		edges[i] = NewWithEndPoints(&ocoord, &dcoords[i])
	}
	if len(edges) == 1 {
		return edges[0]
	}
	for i := 1; i < len(edges); i++ {
		Splice(edges[i-1], edges[i])
	}
	return edges[0]
}
