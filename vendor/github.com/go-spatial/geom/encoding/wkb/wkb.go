//Package wkb is for decoding ESRI's Well Known Binary (WKB) format for OGC geometry (WKBGeometry)
// sepcification at http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm
// There are a few types supported by the specification. Each general type is in it's own file.
// So, to find the implementation of Point (and MultiPoint) it will be located in the point.go
// file. Each of the basic type here adhere to the geom.Geometry interface. So, a wkb point
// is, also, a geom.Point
package wkb

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb/internal/consts"
	"github.com/go-spatial/geom/encoding/wkb/internal/decode"
	"github.com/go-spatial/geom/encoding/wkb/internal/encode"
)

type ErrUnknownGeometryType struct {
	Typ uint32
}

func (e ErrUnknownGeometryType) Error() string {
	return fmt.Sprintf("Unknown Geometry Type %v", e.Typ)
}

//  geometry types
// http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm
const (
	Point           = consts.Point
	LineString      = consts.LineString
	Polygon         = consts.Polygon
	MultiPoint      = consts.MultiPoint
	MultiLineString = consts.MultiLineString
	MultiPolygon    = consts.MultiPolygon
	Collection      = consts.Collection
)

// DecodeBytes will attempt to decode a geometry encoded as WKB into a geom.Geometry.
func DecodeBytes(b []byte) (geo geom.Geometry, err error) {
	buff := bytes.NewReader(b)
	return Decode(buff)
}

// Decode will attempt to decode a geometry encoded as WKB into a geom.Geometry.
func Decode(r io.Reader) (geo geom.Geometry, err error) {

	bom, typ, err := decode.ByteOrderType(r)
	if err != nil {
		return nil, err
	}
	switch typ {
	case Point:
		pt, err := decode.Point(r, bom)
		return geom.Point(pt), err
	case MultiPoint:
		mpt, err := decode.MultiPoint(r, bom)
		return geom.MultiPoint(mpt), err
	case LineString:
		ln, err := decode.LineString(r, bom)
		return geom.LineString(ln), err
	case MultiLineString:
		mln, err := decode.MultiLineString(r, bom)
		return geom.MultiLineString(mln), err
	case Polygon:
		pl, err := decode.Polygon(r, bom)
		return geom.Polygon(pl), err
	case MultiPolygon:
		mpl, err := decode.MultiPolygon(r, bom)
		return geom.MultiPolygon(mpl), err
	case Collection:
		col, err := decode.Collection(r, bom)
		return col, err
	default:
		return nil, ErrUnknownGeometryType{typ}
	}
}

func _encode(en *encode.Encoder, g geom.Geometry) error {
	switch geo := g.(type) {
	case geom.Pointer:
		en.Point(geo.XY())
	case geom.MultiPointer:
		en.MultiPoint(geo.Points())
	case geom.LineStringer:
		en.LineString(geo.Verticies())
	case geom.MultiLineStringer:
		en.MultiLineString(geo.LineStrings())
	case geom.Polygoner:
		en.Polygon(geo.LinearRings())
	case geom.MultiPolygoner:
		en.MultiPolygon(geo.Polygons())
	case geom.Collectioner:
		geoms := geo.Geometries()
		en.BOM().Write(Collection, uint32(len(geoms)))
		for _, gg := range geoms {
			if err := _encode(en, gg); err != nil {
				return err
			}
		}
	default:
		return geom.ErrUnknownGeometry{g}
	}
	return en.Err()
}

func EncodeBytes(g geom.Geometry) (bs []byte, err error) {
	buff := new(bytes.Buffer)
	if err = Encode(buff, g); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func Encode(w io.Writer, g geom.Geometry) error {
	en := encode.Encoder{W: w}
	return _encode(&en, g)
}
