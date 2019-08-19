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
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar/triangulate/quadedge"
)

/*
Segment models a constraint segment in a triangulation.

A constraint segment is an oriented straight line segment between a start point
and an end point.

Author David Skea
Author Martin Davis
Ported to Go by Jason R. Surratt
*/
type Segment struct {
	ls   geom.Line
	data interface{}
}

func NewSegment(l geom.Line) Segment {
	return Segment{ls: l}
}

/*
This makes a deep copy of the line segment, but not the data.

If seg is nil a panic will occur.
*/
func (seg *Segment) DeepCopy() Segment {
	cp := Segment{}
	cp.ls[0][0] = seg.ls[0][0]
	cp.ls[0][1] = seg.ls[0][1]
	cp.ls[1][0] = seg.ls[1][0]
	cp.ls[1][1] = seg.ls[1][1]
	return cp
}

/**
   * Creates a new instance for the given ordinates.
  public Segment(double x1, double y1, double z1, double x2, double y2, double z2) {
    this(new Coordinate(x1, y1, z1), new Coordinate(x2, y2, z2));
  }
*/

/**
   * Creates a new instance for the given ordinates,  with associated external data.
  public Segment(double x1, double y1, double z1, double x2, double y2, double z2, Object data) {
    this(new Coordinate(x1, y1, z1), new Coordinate(x2, y2, z2), data);
  }
*/

/**
   * Creates a new instance for the given points.
   *
   * @param p0 the start point
   * @param p1 the end point
  public Segment(Coordinate p0, Coordinate p1) {
      ls = new LineSegment(p0, p1);
  }
*/

/*
Gets the start coordinate of the segment

Returns the starting vertex

If seg is nil a panic will occur.
*/
func (seg *Segment) GetStart() quadedge.Vertex {
	return quadedge.Vertex(seg.ls[0])
}

/*
Gets the end coordinate of the segment

Return a Coordinate

If seg is nil a panic will occur.
*/
func (seg *Segment) GetEnd() quadedge.Vertex {
	return quadedge.Vertex(seg.ls[1])
}

/**
   * Gets the start X ordinate of the segment
   *
   * @return the X ordinate value
  public double getStartX() {
      Coordinate p = ls.getCoordinate(0);
      return p.x;
  }
*/

/**
   * Gets the start Y ordinate of the segment
   *
   * @return the Y ordinate value
  public double getStartY() {
      Coordinate p = ls.getCoordinate(0);
      return p.y;
  }
*/

/**
   * Gets the start Z ordinate of the segment
   *
   * @return the Z ordinate value
  public double getStartZ() {
      Coordinate p = ls.getCoordinate(0);
      return p.z;
  }
*/

/**
   * Gets the end X ordinate of the segment
   *
   * @return the X ordinate value
  public double getEndX() {
      Coordinate p = ls.getCoordinate(1);
      return p.x;
  }
*/

/**
   * Gets the end Y ordinate of the segment
   *
   * @return the Y ordinate value
  public double getEndY() {
      Coordinate p = ls.getCoordinate(1);
      return p.y;
  }
*/

/**
   * Gets the end Z ordinate of the segment
   *
   * @return the Z ordinate value
  public double getEndZ() {
      Coordinate p = ls.getCoordinate(1);
      return p.z;
  }
*/

// GetLineSegment gets a Line modelling this segment.
func (seg *Segment) GetLineSegment() geom.Line {
	return seg.ls
}

/**
   * Gets the external data associated with this segment
   *
   * @return a data object
  public Object getData() {
      return data;
  }
*/

/**
   * Sets the external data to be associated with this segment
   *
   * @param data a data object
  public void setData(Object data) {
      this.data = data;
  }
*/

/**
   * Determines whether two segments are topologically equal.
   * I.e. equal up to orientation.
   *
   * @param s a segment
   * @return true if the segments are topologically equal
  public boolean equalsTopo(Segment s) {
      return ls.equalsTopo(s.getLineSegment());
  }
*/

/**
   * Computes the intersection point between this segment and another one.
   *
   * @param s a segment
   * @return the intersection point, or <code>null</code> if there is none
  public Coordinate intersection(Segment s) {
      return ls.intersection(s.getLineSegment());
  }
*/

/**
   * Computes a string representation of this segment.
   *
   * @return a string
  public String toString() {
      return ls.toString();
  }
*/
