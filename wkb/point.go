package wkb

import (
	"encoding/binary"
	"io"
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

func NewPoint(x, y float64) Point {
	return Point{
		x: x,
		y: y,
	}
}
