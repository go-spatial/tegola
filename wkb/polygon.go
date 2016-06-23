package wkb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/terranodo/tegola"
)

type Polygon []LineString

func (Polygon) Type() uint32 {
	return GeoPolygon
}

func (p *Polygon) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32
	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}
	for i := uint32(0); i < num; i++ {
		var l = new(LineString)
		if err := l.Decode(bom, r); err != nil {
			return err
		}
		*p = append(*p, *l)
	}
	return nil
}

func (p *Polygon) Sublines() (lns []tegola.LineString) {
	for i := range *p {
		lns = append(lns, &((*p)[i]))
	}
	return lns
}

// MultiPolygon holds multiple polygons.
type MultiPolygon []Polygon

// Type of the Geometry
func (MultiPolygon) Type() uint32 {
	return GeoMultiPolygon
}

// Decode decodes the binary representation of a Multipolygon and decodes it into
// a Multipolygon object.
func (mp *MultiPolygon) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32

	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}

	for i := uint32(0); i < num; i++ {
		var p = new(Polygon)
		byteOrder, typ, err := decodeByteOrderType(r)
		if err != nil {
			return err
		}
		if typ != GeoPolygon {
			return fmt.Errorf("Expect Multipolygons to contains polygons; did not find a polygon.")
		}
		if err := p.Decode(byteOrder, r); err != nil {
			return err
		}
		*mp = append(*mp, *p)

	}
	return nil
}

// Polygons return the sub polygons of a Multipolygon.
func (mp *MultiPolygon) Polygons() (pls []tegola.Polygon) {
	if mp == nil || len(*mp) == 0 {
		return pls
	}
	for i := range *mp {
		pls = append(pls, &((*mp)[i]))
	}
	return pls
}
