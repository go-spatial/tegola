package wkb

import (
	"encoding/binary"
	"io"
)

type LineString []Point

func (ls *LineString) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32
	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}
	for i := 0; i < int(num); i++ {
		var p Point
		if err := p.Decode(bom, r); err != nil {
			return err
		}
		*ls = append(*ls, p)
	}
	return nil
}

func (_ LineString) Type() uint32 {
	return GeoLineString
}
