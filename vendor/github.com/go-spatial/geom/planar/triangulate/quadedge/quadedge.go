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

import (
	"fmt"

	"github.com/go-spatial/geom/cmp"
)

/*
QuadEdge represents the edge data structure which implements the quadedge
algebra. The quadedge algebra was described in a well-known paper by Guibas
and Stolfi, "Primitives for the manipulation of general subdivisions and the
computation of Voronoi diagrams", ACM Transactions on Graphics, 4(2), 1985,
75-123.

Each edge object is part of a quartet of 4 edges, linked via their rot
references. Any edge in the group may be accessed using a series of rot()
operations. Quadedges in a subdivision are linked together via their next
references. The linkage between the quadedge quartets determines the topology
of the subdivision.

The edge class does not contain separate information for vertices or faces; a
vertex is implicitly defined as a ring of edges (created using the next field).

Author David Skea
Author Martin Davis
Ported to Go by Jason R. Surratt
*/
type QuadEdge struct {
	rot    *QuadEdge
	vertex Vertex
	next   *QuadEdge
	data   interface{}
}

/*
MakeEdge creates a new QuadEdge quartet from {@link Vertex} o to {@link Vertex} d.

o - the origin Vertex
d - the destination Vertex
returns the new QuadEdge quartet
*/
func MakeEdge(o Vertex, d Vertex) *QuadEdge {
	q0 := new(QuadEdge)
	q1 := new(QuadEdge)
	q2 := new(QuadEdge)
	q3 := new(QuadEdge)

	q0.rot = q1
	q1.rot = q2
	q2.rot = q3
	q3.rot = q0

	q0.SetNext(q0)
	q1.SetNext(q3)
	q2.SetNext(q2)
	q3.SetNext(q1)

	base := q0
	base.setOrig(o)
	base.setDest(d)
	base.rot.setOrig(o)
	base.rot.setDest(d)

	return base
}

/*
Connect creates a new QuadEdge connecting the destination of a to the origin of
b, in such a way that all three have the same left face after the
connection is complete. Additionally, the data pointers of the new edge
are set.

Returns the connected edge.
*/
func Connect(a *QuadEdge, b *QuadEdge) *QuadEdge {
	e := MakeEdge(a.Dest(), b.Orig())
	Splice(e, a.LNext())
	Splice(e.Sym(), b)
	return e
}

/*
Splices two edges together or apart.
Splice affects the two edge rings around the origins of a and b, and, independently, the two
edge rings around the left faces of <tt>a</tt> and <tt>b</tt>.
In each case, (i) if the two rings are distinct,
Splice will combine them into one, or (ii) if the two are the same ring, Splice will break it
into two separate pieces. Thus, Splice can be used both to attach the two edges together, and
to break them apart.

a - an edge to splice
b - an edge to splice
*/
func Splice(a *QuadEdge, b *QuadEdge) {
	alpha := a.ONext().Rot()
	beta := b.ONext().Rot()

	t1 := b.ONext()
	t2 := a.ONext()
	t3 := beta.ONext()
	t4 := alpha.ONext()

	a.SetNext(t1)
	b.SetNext(t2)
	alpha.SetNext(t3)
	beta.SetNext(t4)
}

/*
Swap Turns an edge counterclockwise inside its enclosing quadrilateral.

e - the quadedge to turn
*/
func Swap(e *QuadEdge) {
	a := e.OPrev()
	b := e.Sym().OPrev()
	Splice(e, a)
	Splice(e.Sym(), b)
	Splice(e, a.LNext())
	Splice(e.Sym(), b.LNext())
	e.setOrig(a.Dest())
	e.setDest(b.Dest())
}

/*
Quadedges must be made using {@link makeEdge},
to ensure proper construction.
private QuadEdge()
{

}
*/

/*
getPrimary gets the primary edge of this quadedge and its sym. The primary
edge is the one for which the origin and destination coordinates are ordered
according to the standard Point ordering.

Returns the primary quadedge

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) GetPrimary() *QuadEdge {
	v1 := qe.Orig()
	v2 := qe.Dest()
	if cmp.PointLess(v1, v2) || cmp.PointEqual(v1, v2) {
		return qe
	}
	return qe.Sym()
}

/*
SetData sets the external data value for this edge.

data an object containing external data

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) SetData(data interface{}) {
	qe.data = data
}

/*
GetData returns the external data value for this edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) GetData() interface{} {
	return qe.data
}

/*
Delete marks this quadedge as being deleted. This does not free the memory
used by this quadedge quartet, but indicates that this edge no longer
participates in a subdivision.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) Delete() {
	qe.rot = nil
}

/*
IsLive tests whether this edge has been deleted.

Returns true if this edge has not been deleted.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) IsLive() bool {
	return qe.rot != nil
}

/*
SetNext sets the connected edge

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) SetNext(next *QuadEdge) {
	qe.next = next
}

/**************************************************************************
QuadEdge Algebra
 ***************************************************************************
*/

/*
Rot gets the dual of this edge, directed from its right to its left.

Return the rotated edge

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) Rot() *QuadEdge {
	return qe.rot
}

/*
InvRot gets the dual of this edge, directed from its left to its right.

Return the inverse rotated edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) InvRot() *QuadEdge {
	return qe.rot.Sym()
}

/*
Sym gets the edge from the destination to the origin of this edge.

Return the sym of the edge

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) Sym() *QuadEdge {
	return qe.rot.rot
}

/*
ONext gets the next CCW edge around the origin of this edge.

Return the next linked edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) ONext() *QuadEdge {
	return qe.next
}

/*
OPrev gets the next CW edge around (from) the origin of this edge.

Return the previous edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) OPrev() *QuadEdge {
	return qe.rot.next.rot
}

/*
DNext gets the next CCW edge around (into) the destination of this edge.

Return the next destination edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) DNext() *QuadEdge {
	return qe.Sym().ONext().Sym()
}

/*
DPrev gets the next CW edge around (into) the destination of this edge.

Return the previous destination edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) DPrev() *QuadEdge {
	return qe.InvRot().ONext().InvRot()
}

/*
LNext gets the CCW edge around the left face following this edge.

Return the next left face edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) LNext() *QuadEdge {
	return qe.InvRot().ONext().Rot()
}

/*
LPrev gets the CCW edge around the left face before this edge.

Return the previous left face edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) LPrev() *QuadEdge {
	return qe.next.Sym()
}

/*
RNext gets the edge around the right face ccw following this edge.

Return the next right face edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) RNext() *QuadEdge {
	return qe.rot.next.InvRot()
}

/*
RPrev gets the edge around the right face ccw before this edge.

Return the previous right face edge.

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) RPrev() *QuadEdge {
	return qe.Sym().ONext()
}

/**********************************************************************************************
Data Access
 **********************************************************************************************/

/*
SetOrig sets the vertex for this edge's origin

o - the origin vertex

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) setOrig(o Vertex) {
	qe.vertex = o
}

/*
SetDest sets the vertex for this edge's destination

d - the destination vertex

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) setDest(d Vertex) {
	qe.Sym().setOrig(d)
}

/*
Orig gets the vertex for the edge's origin

Returns the origin vertex

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) Orig() Vertex {
	return qe.vertex
}

/*
Dest gets the vertex for the edge's destination

Returns the destination vertex

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) Dest() Vertex {
	return qe.Sym().Orig()
}

/*
Gets the length of the geometry of this quadedge.

@return the length of the quadedge
public double getLength() {
    return orig().getCoordinate().distance(dest().getCoordinate());
}
*/

/*
Tests if this quadedge and another have the same line segment geometry,
regardless of orientation.

@param qe a quadedge
@return true if the quadedges are based on the same line segment regardless of orientation
public boolean equalsNonOriented(QuadEdge qe) {
    if (equalsOriented(qe))
        return true;
    if (equalsOriented(qe.sym()))
        return true;
    return false;
}
*/

/*
Tests if this quadedge and another have the same line segment geometry
with the same orientation.

@param qe a quadedge
@return true if the quadedges are based on the same line segment
public boolean equalsOriented(QuadEdge qe) {
    if (orig().getCoordinate().equals2D(qe.orig().getCoordinate())
            && dest().getCoordinate().equals2D(qe.dest().getCoordinate()))
        return true;
    return false;
}
*/

/*
Creates a {@link LineSegment} representing the
geometry of this edge.

@return a LineSegment
public LineSegment toLineSegment()
{
	return new LineSegment(vertex.getCoordinate(), dest().getCoordinate());
}
*/

/*
String Converts this edge to a WKT two-point LINESTRING indicating
the geometry of this edge.

Unlike JTS, if IsLive() is false, a deleted string is returned.

return a String representing this edge's geometry

If qe is nil a panic will occur.
*/
func (qe *QuadEdge) String() string {
	if qe.IsLive() == false {
		return fmt.Sprintf("<deleted %v>", qe.Orig())
	}
	return fmt.Sprintf("LINESTRING (%v %v, %v %v)", qe.Orig().X(), qe.Orig().Y(), qe.Dest().X(), qe.Dest().Y())
}
