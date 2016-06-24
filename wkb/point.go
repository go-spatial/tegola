package wkb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/terranodo/tegola"
)

type Point struct {
	x float64
	y float64
}

func (p *Point) X() float64 {
	if p == nil {
		return 0
	}
	return p.x
}

func (p *Point) Y() float64 {
	if p == nil {
		return 0
	}
	return p.y
}

func (_ *Point) Type() uint32 {
	return GeoPoint
}

func (p *Point) Decode(bom binary.ByteOrder, r io.Reader) error {
	if err := binary.Read(r, bom, &p.x); err != nil {
		return err
	}
	if err := binary.Read(r, bom, &p.y); err != nil {
		return err
	}
	return nil
}
func (p *Point) String() string {
	s, _ := WKT(p) // If we have a failure we don't care
	return s
}

func NewPoint(x, y float64) Point {
	return Point{
		x: x,
		y: y,
	}
}

type MultiPoint []Point

func (MultiPoint) Type() uint32 {
	return GeoMultiPoint
}

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

func (mp *MultiPoint) Points() (pts []tegola.Point) {
	for i := range *mp {
		pts = append(pts, &(*mp)[i])
	}
	return pts
}

func (mp *MultiPoint) String() string {
	s, _ := WKT(mp) // If we have a failure we don't care
	return s
}
