package wkb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/terranodo/tegola"
)

type LineString []Point

func (LineString) Type() uint32 {
	return GeoLineString
}

func (ls *LineString) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32
	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}
	for i := 0; i < int(num); i++ {
		var p = new(Point)
		if err := p.Decode(bom, r); err != nil {
			return err
		}
		*ls = append(*ls, *p)
	}
	return nil
}

func (ls *LineString) Subpoints() (pts []tegola.Point) {
	if ls == nil || len(*ls) == 0 {
		return pts
	}
	for i := range *ls {
		pts = append(pts, &((*ls)[i]))
	}
	return pts
}

type MultiLineString []LineString

func (MultiLineString) Type() uint32 {
	return GeoMultiLineString
}

func (ml *MultiLineString) Lines() (lns []tegola.LineString) {
	if ml == nil || len(*ml) == 0 {
		return lns
	}
	for i := range *ml {
		lns = append(lns, &((*ml)[i]))
	}
	return lns
}
func (ml *MultiLineString) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32
	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}
	for i := uint32(0); i < num; i++ {
		var l = new(LineString)
		byteOrder, typ, err := decodeByteOrderType(r)
		if err != nil {
			return err
		}
		if typ != GeoLineString {
			return fmt.Errorf("Expect Multilines to contains lines; did not find a line.")
		}
		if err := l.Decode(byteOrder, r); err != nil {
			return err
		}
		*ml = append(*ml, *l)
	}
	return nil
}
