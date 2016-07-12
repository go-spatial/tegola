package wkb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/terranodo/tegola"
)

// LineString describes a line, that is made up of two or more points
type LineString []Point

// Type returns the type constant for a LineString
func (LineString) Type() uint32 {
	return GeoLineString
}

// Decode will decode the binary representation into a LineString Object.
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

// Subpoints returns a copy of the points that make up the Line.
func (ls *LineString) Subpoints() (pts []tegola.Point) {
	if ls == nil || len(*ls) == 0 {
		return pts
	}
	for i := range *ls {
		pts = append(pts, &((*ls)[i]))
	}
	return pts
}

//String returns the WKT representation of the Geometry
func (ls *LineString) String() string {
	return WKT(ls) // If we have a failure we don't care
}

//MultiLineString represents one or more independent lines.
type MultiLineString []LineString

//Type returns the Type constant for a Multiline String.
func (MultiLineString) Type() uint32 {
	return GeoMultiLineString
}

//Lines return the indivual lines in the grouping.
func (ml *MultiLineString) Lines() (lns []tegola.LineString) {
	if ml == nil || len(*ml) == 0 {
		return lns
	}
	for i := range *ml {
		lns = append(lns, &((*ml)[i]))
	}
	return lns
}

//Decode takes a byteOrder and an io.Reader, to decode the stream.
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

//String returns the WKT representation of the Geometry.
func (ml *MultiLineString) String() string {
	return WKT(ml)
}
