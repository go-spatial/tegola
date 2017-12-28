package basic

import (
	"fmt"
	"math"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths"
)

func PointsEqual(p1 Point, p2 Point, delta float64) (equal bool) {
	// Checks if p1 == p2 within floatDelta for each coordinate.
	if math.Abs(p1[0]-p2[0]) < delta && math.Abs(p1[1]-p2[1]) < delta {
		equal = true
	}
	return equal
}

// Point describes a simple 2d point
type Point [2]float64

// Just to make basic collection only usable with basic types.
func (Point) basicType() {}

// AsPt returns the equivalent maths.Pt
func (p *Point) AsPt() maths.Pt {
	if p == nil {
		return maths.Pt{0, 0}
	}
	return maths.Pt{p[0], p[1]}
}

// X is the x coordinate
func (bp Point) X() float64 {
	return bp[0]
}

// Y is the y coordinate
func (bp Point) Y() float64 {
	return bp[1]
}

func (p Point) String() string {
	return fmt.Sprintf("Point(%v,%v)", p[0], p[1])
}

// Point3 describes a simple 3d point
type Point3 [3]float64

func Point3sEqual(p1, p2 Point3, delta float64) (equal bool) {
	// Checks if p1 == p2 within floatDelta for each coordinate.
	if math.Abs(p1[0]-p2[0]) < delta && math.Abs(p1[1]-p2[1]) < delta && math.Abs(p1[2]-p2[2]) < delta {
		equal = true
	}
	return equal
}

// Just to make basic collection only usable with basic types.
func (Point3) basicType() {}

// X is the x coordinate
func (bp Point3) X() float64 {
	return bp[0]
}

func (p Point3) String() string {
	return fmt.Sprintf("Point3(%v,%v,%v)", p[0], p[1], p[2])
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

// Checks that mp1 == mp2 with coordinates within delta.
func MultiPointsEqual(mp1, mp2 MultiPoint, delta float64) bool {
	if len(mp1) != len(mp2) {
		return false
	}
	for i := 0; i < len(mp1); i++ {
		if !PointsEqual(mp1[i], mp2[i], delta) {
			return false
		}
	}
	return true
}

// Just to make basic collection only usable with basic types.
func (MultiPoint) basicType()     {}
func (MultiPoint) String() string { return "MultiPoint" }

// Points are the points that make up the set
func (v MultiPoint) Points() (points []tegola.Point) {
	for i := range v {
		points = append(points, v[i])
	}
	return points
}

// MultiPoint3 describes a simple set of 3d points
type MultiPoint3 []Point3

// Checks that mp3a == mp3b with coordinates within delta.
func MultiPoint3sEqual(mp3a, mp3b MultiPoint3, delta float64) bool {
	if len(mp3a) != len(mp3b) {
		return false
	}
	for i := 0; i < len(mp3a); i++ {
		if !Point3sEqual(mp3a[i], mp3b[i], delta) {
			return false
		}
	}
	return true
}

// Just to make basic collection only usable with basic types.
func (MultiPoint3) basicType() {}

// Points are the points that make up the set
func (v MultiPoint3) Points() (points []tegola.Point3) {
	for i := range v {
		points = append(points, v[i])
	}
	return points
}

func (MultiPoint3) String() string {
	return "MultiPoint3"
}
