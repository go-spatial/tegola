package wkb

import (
	"encoding/binary"
	"io"
)

type Point struct {
	X float64
	Y float64
}

func (_ *Point) Type() uint32 {
	return GeoPoint
}

func (p *Point) Decode(bom binary.ByteOrder, r io.Reader) error {
	if err := binary.Read(r, bom, &p.X); err != nil {
		return err
	}
	if err := binary.Read(r, bom, &p.Y); err != nil {
		return err
	}
	return nil
}
