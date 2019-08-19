/*
Copyright (c) 2016 Vivid Solutions.

All rights reserved. This program and the accompanying materials
are made available under the terms of the Eclipse Public License v1.0
and Eclipse Distribution License v. 1.0 which accompanies this distribution.
The Eclipse Public License is available at http://www.eclipse.org/legal/epl-v10.html
and the Eclipse Distribution License is available at

http://www.eclipse.org/org/documents/edl-v10.php.
*/

package quadedge

/*
LastFoundQuadEdgeLocator Locates QuadEdges in a QuadEdgeSubdivision,
optimizing the search by starting in the locality of the last edge found.

Implements the QuadEdgeLocator interface.

Author Martin Davis
Ported to Go by Jason R. Surratt
*/
type LastFoundQuadEdgeLocator struct {
	subdiv   *QuadEdgeSubdivision
	lastEdge *QuadEdge
}

func NewLastFoundQuadEdgeLocator(subdiv *QuadEdgeSubdivision) *LastFoundQuadEdgeLocator {
	var lf LastFoundQuadEdgeLocator

	if subdiv == nil {
		return nil
	}

	lf.subdiv = subdiv
	lf.init()
	return &lf
}

/*
If lf is nil a panic will occur.
*/
func (lf *LastFoundQuadEdgeLocator) init() {
	lf.lastEdge = lf.findEdge()
}

func (lf *LastFoundQuadEdgeLocator) findEdge() *QuadEdge {
	edges := lf.subdiv.GetEdges()
	// assume there is an edge - otherwise will get an exception
	return edges[0]
}

/*
Locate an edge e, such that either v is on e, or e is an edge of a triangle
containing v. The search starts from the last located edge and proceeds on the
general direction of v.
*/
func (lf *LastFoundQuadEdgeLocator) Locate(v Vertex) (*QuadEdge, error) {
	if !lf.lastEdge.IsLive() {
		lf.init()
	}

	e, err := lf.subdiv.LocateFromEdge(v, lf.lastEdge)
	if err != nil {
		return nil, err
	}
	lf.lastEdge = e
	return lf.lastEdge, nil
}
