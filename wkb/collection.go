package wkb

import (
	"encoding/binary"
	"io"
)

//
/*
WKBGeometryCollection {
byte             byte_order;
uint32        wkbType;                       // 7
uint32        num_wkbGeometries;
WKBGeometry      wkbGeometries[num_wkbGeometries]
}
*/

// Collection is a collection of geometries.
type Collection []Geometry

// Type returns the type number of this geometry, by the spec it's 7.
func (Collection) Type() uint32 {
	return GeoGeometryCollection
}

// Decode decodes the geometry from a binary representation.
func (col *Collection) Decode(bom binary.ByteOrder, r io.Reader) error {
	var num uint32
	if err := binary.Read(r, bom, &num); err != nil {
		return err
	}
	for i := uint32(0); i < num; i++ {
		geo, err := Decode(r)
		if err != nil {
			return err
		}
		*col = append(*col, geo)
	}
	return nil
}

func (col *Collection) String() string {
	return WKT(col) // If we have a failure we don't care
}
