package wkb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

//Point is a basic type, this describes a 2D point.
type Point struct {
	basic.Point
}

//Type returns the type constant for this Geometry.
func (*Point) Type() uint32 {
	return GeoPoint
}

//Decode decodes the byte stream into the object.
func (p *Point) Decode(bom binary.ByteOrder, r io.Reader) error {
	if err := binary.Read(r, bom, &p.Point[0]); err != nil {
		return err
	}
	if err := binary.Read(r, bom, &p.Point[1]); err != nil {
		return err
	}
	return nil
}
func (p *Point) String() string {
	return WKT(p) // If we have a failure we don't care
}

//NewPoint creates a new point structure.
func NewPoint(x, y float64) Point {
	return Point{basic.Point{x, y}}
}

//MultiPoint holds one or more independent points in a group
type MultiPoint []Point

//Type returns the type constant for a Mulipoint geometry
func (MultiPoint) Type() uint32 {
	return GeoMultiPoint
}

//Decode decodes the byte stream in the a grouping of points.
func (mp *MultiPoint) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32 // Number of points.
	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}
	for i := uint32(0); i < num; i++ {
		p := new(Point)
		byteOrder, typ, err := decodeByteOrderType(r)
		if err != nil {
			return err
		}
		if typ != GeoPoint {
			return fmt.Errorf("Expect Multipoint to contains points; did not find a point.")
		}
		if err := p.Decode(byteOrder, r); err != nil {
			return err
		}
		*mp = append(*mp, *p)
	}
	return nil
}

//Points returns a copy of the points in the group.
func (mp *MultiPoint) Points() (pts []tegola.Point) {
	for i := range *mp {
		pts = append(pts, &(*mp)[i])
	}
	return pts
}

//String returns the WTK version of the geometry.
func (mp *MultiPoint) String() string {
	return WKT(mp) // If we have a failure we don't care
}
