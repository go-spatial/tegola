//	package for decoding ESRI's Well Known Binary (WKB) format for OGC geometry (WKBGeometry)
//	sepcification at http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm
package wkb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/terranodo/tegola"
)

//  geometry types
// http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm
const (
	GeoPoint              uint32 = 1
	GeoLineString                = 2
	GeoPolygon                   = 3
	GeoMultiPoint                = 4
	GeoMultiLineString           = 5
	GeoMultiPolygon              = 6
	GeoGeometryCollection        = 7
)

type Geometry interface {
	Decode(bom binary.ByteOrder, r io.Reader) error
	Type() uint32
}

func decodeByteOrderType(r io.Reader) (byteOrder binary.ByteOrder, typ uint32, err error) {
	var bom = make([]byte, 1, 1)
	// the bom is the first byte
	if _, err := r.Read(bom); err != nil {
		return byteOrder, typ, err
	}

	if bom[0] == 0 {
		byteOrder = binary.BigEndian
	} else {
		byteOrder = binary.LittleEndian
	}

	// Reading the type which is 4 bytes
	err = binary.Read(r, byteOrder, &typ)
	return byteOrder, typ, err

}

// TODO
func Encode(w io.Writer, bom binary.ByteOrder, geometry tegola.Geometry) error {
	// TODO
	return nil
}
func WKB(geometry tegola.Geometry) (geo Geometry, err error) {
	// TODO
	return nil, nil
}
func Decode(r io.Reader) (geo Geometry, err error) {

	byteOrder, typ, err := decodeByteOrderType(r)

	if err != nil {
		return nil, err
	}
	switch typ {
	case GeoPoint:
		geo = new(Point)
	case GeoMultiPoint:
	case GeoLineString:
		geo = new(LineString)
	default:
		return nil, fmt.Errorf("Unknown Geometry! %v", typ)
	}
	if err := geo.Decode(byteOrder, r); err != nil {
		return nil, err
	}
	return geo, nil
}
