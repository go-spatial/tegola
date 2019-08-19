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
	"github.com/go-spatial/geom/planar/triangulate/quadedge"
)

/*
IncrementalDelaunayTriangulator computes a Delaunay Triangulation of a set of
{@link Vertex}es, using an incremental insertion algorithm.

Author Martin Davis
Ported to Go by Jason R. Surratt
*/
type IncrementalDelaunayTriangulator struct {
	subdiv *quadedge.QuadEdgeSubdivision
}

/*
InsertSites inserts all sites in a collection. The inserted vertices MUST be
unique up to the provided tolerance value. (i.e. no two vertices should be
closer than the provided tolerance value). They do not have to be rounded
to the tolerance grid, however.

vertices - a Collection of Vertex
Returns ErrLocateFailure if the location algorithm fails to converge in a
reasonable number of iterations. If this occurs the triangulator is left in
an unknown state with 0 or more of the vertices inserted.

If idt is nil a panic will occur.
*/
func (idt *IncrementalDelaunayTriangulator) InsertSites(vertices []quadedge.Vertex) error {
	for _, v := range vertices {
		_, err := idt.InsertSite(v)
		if err != nil {
			return err
		}
	}

	return nil
}

/*
InsertSite inserts a new point into a subdivision representing a Delaunay
triangulation, and fixes the affected edges so that the result is still a
Delaunay triangulation.

Returns a tuple with a quadedge containing the inserted vertex and an error
code. If there is an error then the vertex will not be inserted and the
triangulator will still be in a consistent state.

If idt is nil a panic will occur.
*/
func (idt *IncrementalDelaunayTriangulator) InsertSite(v quadedge.Vertex) (*quadedge.QuadEdge, error) {

	// log.Printf("Inserting: %v", v);
	// log.Printf("Initial: %v", idt.subdiv.DebugDumpEdges())
	/*
		This code is based on Guibas and Stolfi (1985), with minor modifications
		and a bug fix from Dani Lischinski (Graphic Gems 1993). (The modification
		I believe is the test for the inserted site falling exactly on an
		existing edge. Without this test zero-width triangles have been observed
		to be created)
	*/
	e, err := idt.subdiv.Locate(v)
	// log.Printf("e: %v -> %v", e.Orig(), e.Dest())
	if err != nil {
		return nil, err
	}

	if idt.subdiv.IsVertexOfEdge(e, v) {
		// log.Printf("On Vertex");
		// point is already in subdivision.
		return e, nil
	}
	if idt.subdiv.IsOnEdge(e, v) {
		// log.Printf("On Edge");
		// the point lies exactly on an edge, so delete the edge
		// (it will be replaced by a pair of edges which have the point as a vertex)
		e = e.OPrev()
		idt.subdiv.Delete(e.ONext())
	}

	/*
		Connect the new point to the vertices of the containing triangle
		(or quadrilateral, if the new point fell on an existing edge.)
	*/
	base := quadedge.MakeEdge(e.Orig(), v)
	// log.Printf("Made Edge: %v -> %v", base.Orig(), base.Dest());
	quadedge.Splice(base, e)
	startEdge := base

	for {
		base = idt.subdiv.Connect(e, base.Sym())
		e = base.OPrev()
		if e.LNext() == startEdge {
			break
		}
	}

	// Examine suspect edges to ensure that the Delaunay condition
	// is satisfied.
	for {
		t := e.OPrev()
		if t.Dest().RightOf(*e) && v.IsInCircle(e.Orig(), t.Dest(), e.Dest()) {
			quadedge.Swap(e)
			e = e.OPrev()
		} else if e.ONext() == startEdge {
			// log.Printf("New State: %v", idt.subdiv.DebugDumpEdges())
			return base, nil // no more suspect edges.
		} else {
			e = e.ONext().LPrev()
		}
	}
}
