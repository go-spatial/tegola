package basic

import (
	"fmt"

	"github.com/terranodo/tegola"
)

// Point describes a simple 2d point
type Point [2]float64

// Just to make basic collection only usable with basic types.
func (Point) basicType() {}

// X is the x coordinate
func (bp Point) X() float64 {
	return bp[0]
}

// Y is the y coordinate
func (bp Point) Y() float64 {
	return bp[1]
}

func (p *Point) String() string {
	if p == nil {
		return "Point(nil)"
	}
	return fmt.Sprintf("Point(%v %v)", p.X(), p.Y())
}

// Point3 describes a simple 3d point
type Point3 [3]float64

// Just to make basic collection only usable with basic types.
func (Point3) basicType() {}

// X is the x coordinate
func (bp Point3) X() float64 {
	return bp[0]
}
func (Point3) String() string {
	return "Point3"
}

// Y is the y coordinate
func (bp Point3) Y() float64 {
	return bp[1]
}

// Z is the z coordinate
func (bp Point3) Z() float64 {
	return bp[2]
}

// MultiPoint describes a simple set of 2d points
type MultiPoint []Point

// Just to make basic collection only usable with basic types.
func (MultiPoint) basicType() {}
func (MultiPoint) String() string {
	return "MultiPoint"
}

// Points are the points that make up the set
func (v MultiPoint) Points() (points []tegola.Point) {
	for i := range v {
		points = append(points, v[i])
	}
	return points
}

// MultiPoint3 describes a simple set of 3d points
type MultiPoint3 []Point3

// Just to make basic collection only usable with basic types.
func (MultiPoint3) basicType() {}

// Points are the points that make up the set
func (v MultiPoint3) Points() (points []tegola.Point) {
	for i := range v {
		points = append(points, v[i])
	}
	return points
}

func (MultiPoint3) String() string {
	return "MultiPoint3"
}
