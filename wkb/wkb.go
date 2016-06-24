//	Package wkb is for decoding ESRI's Well Known Binary (WKB) format for OGC geometry (WKBGeometry)
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

func encode(bom binary.ByteOrder, geometry tegola.Geometry) (data []interface{}) {

	if bom == binary.LittleEndian {
		data = append(data, byte(1))
	} else {
		data = append(data, byte(0))
	}
	switch geo := geometry.(type) {
	default:
		return nil
	case tegola.Point:
		data = append(data, GeoPoint)
		data = append(data, geo.X(), geo.Y())
		return data
	case tegola.MultiPoint:
		data = append(data, GeoMultiPoint)
		pts := geo.Points()
		if len(pts) == 0 {
			return data
		}
		for _, p := range pts {
			pd := encode(bom, p)
			if pd == nil {
				return nil
			}
			data = append(data, pd...)
		}
		return data
	case tegola.LineString:
		data = append(data, GeoLineString)
		pts := geo.Subpoints()
		data = append(data, uint32(len(pts))) // Number of points in the line string
		for i := range pts {
			data = append(data, pts[i]) // The points.
		}
		return data

	case tegola.MultiLine:
		data = append(data, GeoMultiLineString)
		lns := geo.Lines()
		data = append(data, uint32(len(lns))) // Number of lines in the Multi line string
		for _, l := range lns {
			ld := encode(bom, l)
			if ld == nil {
				return nil
			}
			data = append(data, ld...)
		}
		return data

	case tegola.Polygon:
		data = append(data, GeoPolygon)
		lns := geo.Sublines()
		data = append(data, uint32(len(lns))) // Number of rings in the polygon
		for i := range lns {
			pts := lns[i].Subpoints()
			data = append(data, uint32(len(pts))) // Number of points in the ring
			for i := range pts {
				data = append(data, pts[i]) // The points in the ring
			}
		}
		return data
	case tegola.MultiPolygon:
		data = append(data, GeoMultiPolygon)
		pls := geo.Polygons()
		data = append(data, uint32(len(pls))) // Number of Polygons in the Multi.
		for _, p := range pls {
			pd := encode(bom, p)
			if pd == nil {
				return nil
			}
			data = append(data, pd...)
		}
		return data
	case tegola.Collection:
		data = append(data, GeoGeometryCollection)
		geometries := geo.Geometries()
		data = append(data, uint32(len(geometries))) // Number of Geometries
		for _, g := range geometries {
			gd := encode(bom, g)
			if gd == nil {
				return nil
			}
			data = append(data, gd...)
		}
		return data
	}
}

func Encode(w io.Writer, bom binary.ByteOrder, geometry tegola.Geometry) error {
	data := encode(bom, geometry)
	if data == nil {
		return fmt.Errorf("Unabled to encode %v", geometry)
	}
	return binary.Write(w, bom, data)
}

func WKB(geometry tegola.Geometry) (geo Geometry, err error) {
	switch geo := geometry.(type) {
	case tegola.Point:
		p := NewPoint(geo.X(), geo.Y())
		return &p, nil
	case tegola.Point3: // Not supported.
	case tegola.LineString:
		l := LineString{}
		for _, p := range geo.Subpoints() {
			l = append(l, NewPoint(p.X(), p.Y()))
		}
		return &l, nil
	case tegola.MultiLine:
		ml := MultiLineString{}
		for _, l := range geo.Lines() {
			g, err := WKB(l)
			if err != nil {
				return nil, err
			}
			lg, ok := g.(*LineString)
			if !ok {
				return nil, fmt.Errorf("Was not able to convert to LineString: %v", lg)
			}
			ml = append(ml, *lg)
		}
		return &ml, nil
	case tegola.Polygon:
		p := Polygon{}
		for _, l := range geo.Sublines() {
			g, err := WKB(l)
			if err != nil {
				return nil, err
			}
			lg, ok := g.(*LineString)
			if !ok {
				return nil, fmt.Errorf("Was not able to convert to LineString: %v", lg)
			}
			p = append(p, *lg)
		}
		return &p, nil
	case tegola.MultiPolygon:
		mp := MultiPolygon{}
		for _, p := range geo.Polygons() {
			g, err := WKB(p)
			if err != nil {
				return nil, err
			}
			pg, ok := g.(*Polygon)
			if !ok {
				return nil, fmt.Errorf("Was not able to convert to Polygon: %v", g)
			}
			mp = append(mp, *pg)
		}
		return &mp, nil
	case tegola.Collection:
		col := Collection{}
		for _, c := range geo.Geometries() {
			g, err := WKB(c)
			if err != nil {
				return nil, err
			}
			cg, ok := g.(Geometry)
			if !ok {
				return nil, fmt.Errorf("Was not able to convert to a Geometry type: %v", cg)
			}
			col = append(col, cg)
		}
		return &col, nil
	}
	return nil, fmt.Errorf("Not supported")
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
		geo = new(MultiPoint)
	case GeoLineString:
		geo = new(LineString)
	case GeoMultiLineString:
		geo = new(MultiLineString)
	case GeoMultiPolygon:
		geo = new(MultiPolygon)
	default:
		return nil, fmt.Errorf("Unknown Geometry! %v", typ)
	}
	if err := geo.Decode(byteOrder, r); err != nil {
		return nil, err
	}
	return geo, nil
}
