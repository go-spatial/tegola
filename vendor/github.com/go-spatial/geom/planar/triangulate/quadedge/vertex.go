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
	"encoding/json"
	"math"
)

const (
	LEFT = iota
	RIGHT
	BEYOND
	BEHIND
	BETWEEN
	ORIGIN
	DESTINATION
)

/*
Vertex models a site (node) in a QuadEdgeSubdivision. The sites can be points
on a line string representing a linear site.

The vertex can be considered as a vector with a norm, length, inner product,
cross product, etc. Additionally, point relations (e.g., is a point to the
left of a line, the circle defined by this point and two others, etc.) are
also defined in this class.

Author David Skea
Author Martin Davis
Ported to Go by Jason R. Surratt
*/
type Vertex [2]float64

// XY implements the geom.Pointer interface
func (u Vertex) XY() [2]float64 { return u }
func (u Vertex) X() float64     { return u[0] }
func (u Vertex) Y() float64     { return u[1] }

func (u Vertex) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		X float64
		Y float64
	}{
		u.X(),
		u.Y(),
	})
}

func (u Vertex) Equals(other Vertex) bool {
	if u.X() == other.X() && u.Y() == other.Y() {
		return true
	}
	return false
}

func (u Vertex) EqualsTolerance(other Vertex, tolerance float64) bool {
	v1 := u[0] - other[0]
	v2 := u[1] - other[1]
	if math.Sqrt(v1*v1+v2*v2) < tolerance {
		return true
	}
	return false
}

func (u Vertex) Classify(p0 Vertex, p1 Vertex) int {
	p2 := u
	a := p1.Sub(p0)
	b := p2.Sub(p0)
	sa := a.CrossProduct(b)

	switch {
	case sa > 0.0:
		return LEFT
	case sa < 0.0:
		return RIGHT
	case a.X()*b.X() < 0.0 || a.Y()*b.Y() < 0.0:
		return BEHIND
	case a.Magn() < b.Magn():
		return BEYOND
	case p0.Equals(p2):
		return ORIGIN
	case p1.Equals(p2):
		return DESTINATION
	default:
		return BETWEEN
	}
}

/*
CrossProduct computes the cross product k = u X v.

@param v a vertex
@return returns the magnitude of u X v
*/
func (u Vertex) CrossProduct(v Vertex) float64 {
	return (u.X()*v.Y() - u.Y()*v.X())
}

/*
Dot computes the inner or dot product

@param v a vertex
@return returns the dot product u.v
*/
func (u Vertex) Dot(v Vertex) float64 {
	return u.X()*v.X() + u.Y()*v.Y()
}

/*
Times computes the scalar product c(v)

Return the scaled vector
*/
func (u Vertex) Times(c float64) Vertex {
	return Vertex{u.X() * c, u.Y() * c}
}

/*
Divide computes the scalar division v / c

Returns the scaled vector

This is not part of the original JTS code.
*/
func (u Vertex) Divide(c float64) Vertex {
	return Vertex{u.X() / c, u.Y() / c}
}

// Sum u + v and return the new Vertex
func (u Vertex) Sum(v Vertex) Vertex {
	return Vertex{u.X() + v.X(), u.Y() + v.Y()}
}

// Sub subtracts u - v and returns the new Vertex
func (u Vertex) Sub(v Vertex) Vertex {
	return Vertex{u.X() - v.X(), u.Y() - v.Y()}
}

// Magn returns the magnitude of the vector
func (u Vertex) Magn() float64 {
	return math.Sqrt(u.X()*u.X() + u.Y()*u.Y())
}

/*
Normalize scales the vector so the length is one.

This is not part of the original JTS code.
*/
func (u Vertex) Normalize() Vertex {
	return u.Divide(u.Magn())
}

/*
Cross returns k X v (cross product). this is a vector perpendicular to v
*/
func (u Vertex) Cross() Vertex {
	return Vertex{u.Y(), -u.X()}
}

/*********************************************************************************************
Geometric primitives /
**********************************************************************************************/

/*
IsInCircle tests if the vertex is inside the circle defined by
the triangle with vertices a, b, c (oriented counter-clockwise).

a - a vertex of the triangle
b - a vertex of the triangle
c - a vertex of the triangle
Return true if this vertex is in the circumcircle of (a,b,c)
*/
func (u Vertex) IsInCircle(a Vertex, b Vertex, c Vertex) bool {
	// vertext version of IsInCircleNormalized from IsInCircleNormalized
	adx := a[0] - u[0]
	ady := a[1] - u[1]
	bdx := b[0] - u[0]
	bdy := b[1] - u[1]
	cdx := c[0] - u[0]
	cdy := c[1] - u[1]

	abdet := adx*bdy - bdx*ady
	bcdet := bdx*cdy - cdx*bdy
	cadet := cdx*ady - adx*cdy
	alift := adx*adx + ady*ady
	blift := bdx*bdx + bdy*bdy
	clift := cdx*cdx + cdy*cdy

	disc := alift*bcdet + blift*cadet + clift*abdet
	return disc > 0
}

/**
IsCCW Tests whether the triangle formed by this vertex and two
other vertices is in CCW orientation.

b - a vertex
c - a vertex
return true if the triangle is oriented CCW
*/
func (u Vertex) IsCCW(b Vertex, c Vertex) bool {
	//  // test code used to check for robustness of triArea
	//  boolean isCCW = (b.p.x - p.x)(c.p.y - p.y)
	//  - (b.p.y - p.y)(c.p.x - p.x) > 0;
	// //boolean isCCW = triArea(this, b, c) > 0;
	// boolean isCCWRobust = CGAlgorithms.orientationIndex(p, b.p, c.p) == CGAlgorithms.COUNTERCLOCKWISE;
	// if (isCCWRobust != isCCW)
	//  System.out.println("CCW failure");

	// is equal to the signed area of the triangle

	return (b.X()-u.X())*(c.Y()-u.Y())-(b.Y()-u.Y())*(c.X()-u.X()) > 0

	// original rolled code
	//boolean isCCW = triArea(this, b, c) > 0;
	//return isCCW;

}

func (u Vertex) RightOf(e QuadEdge) bool {
	return u.IsCCW(e.Dest(), e.Orig())
}

func (u Vertex) LeftOf(e QuadEdge) bool {
	return u.IsCCW(e.Orig(), e.Dest())
}

/*
private HCoordinate bisector(Vertex a, Vertex b) {
    // returns the perpendicular bisector of the line segment ab
    double dx = b.getX() - a.getX();
    double dy = b.getY() - a.getY();
    HCoordinate l1 = new HCoordinate(a.getX() + dx / 2.0, a.getY() + dy / 2.0, 1.0);
    HCoordinate l2 = new HCoordinate(a.getX() - dy + dx / 2.0, a.getY() + dx + dy / 2.0, 1.0);
    return new HCoordinate(l1, l2);
}

private double distance(Vertex v1, Vertex v2) {
    return Math.sqrt(Math.pow(v2.getX() - v1.getX(), 2.0)
            + Math.pow(v2.getY() - v1.getY(), 2.0));
}
*/

/**
Computes the value of the ratio of the circumradius to shortest edge. If smaller than some
given tolerance B, the associated triangle is considered skinny. For an equal lateral
triangle this value is 0.57735. The ratio is related to the minimum triangle angle theta by:
circumRadius/shortestEdge = 1/(2sin(theta)).

@param b second vertex of the triangle
@param c third vertex of the triangle
@return ratio of circumradius to shortest edge.

public double circumRadiusRatio(Vertex b, Vertex c) {
    Vertex x = this.circleCenter(b, c);
    double radius = distance(x, b);
    double edgeLength = distance(this, b);
    double el = distance(b, c);
    if (el < edgeLength) {
        edgeLength = el;
    }
    el = distance(c, this);
    if (el < edgeLength) {
        edgeLength = el;
    }
    return radius / edgeLength;
}
*/

/**
returns a new vertex that is mid-way between this vertex and another end point.

@param a the other end point.
@return the point mid-way between this and that.

public Vertex midPoint(Vertex a) {
    double xm = (p.x + a.getX()) / 2.0;
    double ym = (p.y + a.getY()) / 2.0;
    double zm = (p.z + a.getZ()) / 2.0;
    return new Vertex(xm, ym, zm);
}
*/

/**
Computes the centre of the circumcircle of this vertex and two others.

@param b
@param c
@return the Coordinate which is the circumcircle of the 3 points.

public Vertex circleCenter(Vertex b, Vertex c) {
    Vertex a = new Vertex(this.getX(), this.getY());
    // compute the perpendicular bisector of cord ab
    HCoordinate cab = bisector(a, b);
    // compute the perpendicular bisector of cord bc
    HCoordinate cbc = bisector(b, c);
    // compute the intersection of the bisectors (circle radii)
    HCoordinate hcc = new HCoordinate(cab, cbc);
    Vertex cc = null;
    try {
        cc = new Vertex(hcc.getX(), hcc.getY());
    } catch (NotRepresentableException nre) {
        System.err.println("a: " + a + "  b: " + b + "  c: " + c);
        System.err.println(nre);
    }
    return cc;
}
*/

/**
For this vertex enclosed in a triangle defined by three vertices v0, v1 and v2, interpolate
a z value from the surrounding vertices.

public double interpolateZValue(Vertex v0, Vertex v1, Vertex v2) {
    double x0 = v0.getX();
    double y0 = v0.getY();
    double a = v1.getX() - x0;
    double b = v2.getX() - x0;
    double c = v1.getY() - y0;
    double d = v2.getY() - y0;
    double det = ad - bc;
    double dx = this.getX() - x0;
    double dy = this.getY() - y0;
    double t = (ddx - bdy) / det;
    double u = (-cdx + ady) / det;
    double z = v0.getZ() + t(v1.getZ() - v0.getZ()) + u(v2.getZ() - v0.getZ());
    return z;
}
*/

/**
Interpolates the Z-value (height) of a point enclosed in a triangle
whose vertices all have Z values.
The containing triangle must not be degenerate
(in other words, the three vertices must enclose a
non-zero area).

@param p the point to interpolate the Z value of
@param v0 a vertex of a triangle containing the p
@param v1 a vertex of a triangle containing the p
@param v2 a vertex of a triangle containing the p
@return the interpolated Z-value (height) of the point

public static double interpolateZ(Coordinate p, Coordinate v0, Coordinate v1, Coordinate v2) {
    double x0 = v0.x;
    double y0 = v0.y;
    double a = v1.x - x0;
    double b = v2.x - x0;
    double c = v1.y - y0;
    double d = v2.y - y0;
    double det = ad - bc;
    double dx = p.x - x0;
    double dy = p.y - y0;
    double t = (ddx - bdy) / det;
    double u = (-cdx + ady) / det;
    double z = v0.z + t(v1.z - v0.z) + u(v2.z - v0.z);
    return z;
}
*/

/**
Computes the interpolated Z-value for a point p lying on the segment p0-p1

@param p
@param p0
@param p1
@return the interpolated Z value

public static double interpolateZ(Coordinate p, Coordinate p0, Coordinate p1) {
    double segLen = p0.distance(p1);
    double ptLen = p.distance(p0);
    double dz = p1.z - p0.z;
    double pz = p0.z + dz(ptLen / segLen);
    return pz;
}
*/
