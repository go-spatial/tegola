/*
Copyright (c) 2016 Vivid Solutions.

All rights reserved. This program and the accompanying materials
are made available under the terms of the Eclipse Public License v1.0
and Eclipse Distribution License v. 1.0 which accompanies this distribution.
The Eclipse Public License is available at http://www.eclipse.org/legal/epl-v10.html
and the Eclipse Distribution License is available at

http://www.eclipse.org/org/documents/edl-v10.php.
*/

package triangulate

import (
	"log"
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar/triangulate/quadedge"
)

/*
DelaunayTriangulationBuilder is a utility class which creates Delaunay
Triangulations from collections of points and extract the resulting
triangulation edges or triangles as geometries.

Author Martin Davis
Ported to Go by Jason R. Surratt
*/
type DelaunayTriangulationBuilder struct {
	siteCoords []quadedge.Vertex
	tolerance  float64
	subdiv     *quadedge.QuadEdgeSubdivision
}

type PointByXY []quadedge.Vertex

func (xy PointByXY) Less(i, j int) bool { return cmp.XYLessPoint(xy[i], xy[j]) }
func (xy PointByXY) Swap(i, j int)      { xy[i], xy[j] = xy[j], xy[i] }
func (xy PointByXY) Len() int           { return len(xy) }

func NewDelaunayTriangulationBuilder(tolerance float64) *DelaunayTriangulationBuilder {
	return &DelaunayTriangulationBuilder{tolerance: tolerance}
}

/*
extractUniqueCoordinates extracts the unique points from the given Geometry.

geom - the geometry to extract from
Returns a List of the unique Coordinates

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) extractUniqueCoordinates(g ...geom.Geometry) ([]quadedge.Vertex, error) {

	vertices := make([]quadedge.Vertex, 0)

	for _, g1 := range g {
		if g1 == nil {
			continue
		}

		coords, err := geom.GetCoordinates(g1)
		if err != nil {
			return nil, err
		}

		for i := range coords {
			vertices = append(vertices, quadedge.Vertex{coords[i][0], coords[i][1]})
		}
	}

	return dtb.unique(vertices), nil
}

/*
unique returns a list of unique vertices.

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) unique(points []quadedge.Vertex) []quadedge.Vertex {
	sort.Sort(PointByXY(points))

	// we can use a slice trick to avoid copying the array again. Maybe better
	// than two index variables...
	uniqued := points[:0]
	for i := 0; i < len(points); i++ {
		if i == 0 || cmp.PointEqual(points[i], points[i-1]) == false {
			uniqued = append(uniqued, points[i])
		}
	}

	return uniqued
}

/**
 * Converts all {@link Coordinate}s in a collection to {@link Vertex}es.
 * @param coords the coordinates to convert
 * @return a List of Vertex objects
public static List toVertices(Collection coords)
{
	List verts = new ArrayList();
	for (Iterator i = coords.iterator(); i.hasNext(); ) {
		Coordinate coord = (Coordinate) i.next();
		verts.add(new Vertex(coord));
	}
	return verts;
}
*/

/**
 * Computes the {@link Envelope} of a collection of {@link Coordinate}s.
 *
 * @param coords a List of Coordinates
 * @return the envelope of the set of coordinates
public static Envelope envelope(Collection coords)
{
	Envelope env = new Envelope();
	for (Iterator i = coords.iterator(); i.hasNext(); ) {
		Coordinate coord = (Coordinate) i.next();
		env.expandToInclude(coord);
	}
	return env;
}
*/

/*
SetSites sets the vertices which will be triangulated. All vertices of the
given geometry will be used as sites.

geom - the geometry from which the sites will be extracted.

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) SetSites(g ...geom.Geometry) error {
	// remove any duplicate points (they will cause the triangulation to fail)
	c, err := dtb.extractUniqueCoordinates(g...)
	dtb.siteCoords = c
	if debug {
		log.Printf("siteCoords %v", c)
	}
	return err
}

/**
 * Sets the sites (vertices) which will be triangulated
 * from a collection of {@link Coordinate}s.
 *
 * @param coords a collection of Coordinates.
public void setSites(Collection coords)
{
	// remove any duplicate points (they will cause the triangulation to fail)
	siteCoords = unique(CoordinateArrays.toCoordinateArray(coords));
}
*/

/**
 * Sets the snapping tolerance which will be used
 * to improved the robustness of the triangulation computation.
 * A tolerance of 0.0 specifies that no snapping will take place.
 *
 * @param tolerance the tolerance distance to use
public void setTolerance(double tolerance)
{
	this.tolerance = tolerance;
}
*/

/*
create will create the triangulation.

return true on success, false on failure.

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) create() bool {
	if dtb.subdiv != nil {
		return true
	}
	if len(dtb.siteCoords) == 0 {
		return false
	}

	var siteEnv *geom.Extent
	for _, v := range dtb.siteCoords {
		if siteEnv == nil {
			siteEnv = geom.NewExtent(v)
		}
		siteEnv.AddGeometry(v)
	}

	dtb.subdiv = quadedge.NewQuadEdgeSubdivision(*siteEnv, dtb.tolerance)
	triangulator := new(IncrementalDelaunayTriangulator)
	triangulator.subdiv = dtb.subdiv
	triangulator.InsertSites(dtb.siteCoords)

	return true
}

/*
GetSubdivision gets the QuadEdgeSubdivision which models the computed
triangulation.

Returns the subdivision containing the triangulation or nil if it has
not been created.

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) GetSubdivision() *quadedge.QuadEdgeSubdivision {
	dtb.create()
	return dtb.subdiv
}

/*
GetEdges gets the edges of the computed triangulation as a MultiLineString.

returns the edges of the triangulation

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) GetEdges() geom.MultiLineString {
	if !dtb.create() {
		return geom.MultiLineString{}
	}
	return dtb.subdiv.GetEdgesAsMultiLineString()
}

/*
GetTriangles Gets the faces of the computed triangulation as a MultiPolygon.

Unlike JTS, this method returns a MultiPolygon. I found not all viewers like
displaying collections. -JRS

If dtb is nil a panic will occur.
*/
func (dtb *DelaunayTriangulationBuilder) GetTriangles() (geom.MultiPolygon, error) {
	if !dtb.create() {
		return geom.MultiPolygon{}, nil
	}
	return dtb.subdiv.GetTriangles()
}
