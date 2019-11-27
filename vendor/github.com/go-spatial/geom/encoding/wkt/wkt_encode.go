package wkt

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
)

// Encoder holds the necessary configurations and state for
// a WKT Encoder
type Encoder struct {
	w         io.Writer
	fbuf      []byte
	strict    bool
	precision int
	fmt       byte
}

// NewDefaultEncoder creates a new encoder that writes to w using the
// defaults of strict = false, precision = 10, and fmt = 'g'.
func NewDefaultEncoder(w io.Writer) Encoder {
	return NewEncoder(w, false, 10, 'g')
}

// NewEncoder creates a new encoder that writes to w
//
// strict causes the encoder to return errors if the geometries have empty
// sub geometries where not allowed by the wkt spec. When Strict
// false, empty geometries are ignored.
//		Point: can be empty
//		MultiPoint: can have empty points
//		LineString: cannot have empty points
//		MultiLineString: can have empty line strings non-empty line strings cannot have empty points
//		Polygon: cannot have empty line strings, non-empty line strings cannot have empty points
//		MultiPolygon: can have empty polygons, polygons cannot have empty linestrings, line strings cannot have empty points
//		Collection: can have empty geometries
//
// The precision and fmt are passed into strconv.FormatFloat
// https://golang.org/pkg/strconv/#FormatFloat
func NewEncoder(w io.Writer, strict bool, precision int, fmt byte) Encoder {
	// our float would be one of
	// ddd.ddd with Precision+1 number of d's => Precision + 1 + 1
	// d.ddde+xx with Precision+1 number of d's => Precision + 1 + 5
	return Encoder{
		w:         w,
		fbuf:      make([]byte, 0, precision+6),
		strict:    strict,
		precision: precision,
		fmt:       fmt,
	}
}

func (enc Encoder) byte(b byte) error {
	buf := append(enc.fbuf[:0], b)
	_, err := enc.w.Write(buf)
	return err
}

func (enc Encoder) string(s string) error {
	_, err := enc.w.Write([]byte(s))
	return err
}

func (enc Encoder) formatFloat(f float64) error {
	buf := strconv.AppendFloat(enc.fbuf[:0], f, enc.fmt, enc.precision, 64)
	_, err := enc.w.Write(buf)
	return err
}

func (enc Encoder) encodePair(pt [2]float64) error {
	// should onlt be called for multipoints
	if cmp.IsEmptyPoint(pt) {
		return enc.string("EMPTY")
	}

	err := enc.formatFloat(pt[0])
	if err != nil {
		return err
	}

	err = enc.byte(' ')
	if err != nil {
		return err
	}

	return enc.formatFloat(pt[1])
}

func (enc Encoder) encodePoint(pt [2]float64) error {
	// empty point
	if cmp.IsEmptyPoint(pt) {
		err := enc.string("EMPTY")
		return err
	}

	err := enc.byte('(')
	if err != nil {
		return err
	}

	err = enc.encodePair(pt)
	if err != nil {
		return err
	}

	return enc.byte(')')
}

func lastNonEmptyIdxPoints(mp [][2]float64) (last int) {
	for i := len(mp) - 1; i >= 0; i-- {
		if !cmp.IsEmptyPoint(mp[i]) {
			return i
		}
	}

	return -1
}

func lastNonEmptyIdxLines(lines [][][2]float64) (last int) {
	for i := len(lines) - 1; i >= 0; i-- {
		last := lastNonEmptyIdxPoints(lines[i])
		if last != -1 {
			return i
		}
	}
	return -1
}

func lastNonEmptyIdxPolys(polys [][][][2]float64) (last int) {
	for i := len(polys) - 1; i >= 0; i-- {
		last := lastNonEmptyIdxLines(polys[i])
		if last != -1 {
			return i
		}
	}
	return -1
}

func (enc Encoder) encodePoints(mp [][2]float64, last int, gType byte) (err error) {

	// the last encode point
	var firstEnc *[2]float64
	var lastEnc *[2]float64
	var count int

	for i, v := range mp[:last+1] {
		// if the last point is the same as this point and
		// we aren't encoding a multipoint, then dups should get dropped
		if lastEnc != nil && *lastEnc == v && gType != mpType {
			continue
		}

		if cmp.IsEmptyPoint(v) {
			if enc.strict {
				switch gType {
				case mpType:
					// multipoints can have empty points
					// encodePair will write EMPTY
					break
				case lsType:
					return errors.New("cannot have empty points in strict LINESTRING")
				case mlType:
					return errors.New("cannot have empty points in strict MULTILINESTRING")
				case polyType:
					return errors.New("cannot have empty points in strict POLYGON")
				case mPolyType:
					return errors.New("cannot have empty points in strict MULTIPOLYGON")
				default:
					panic("unreachable")
				}
			} else {
				// multipoints can have empty points
				// encodePair will write EMPTY
				if gType != mpType {
					continue
				}
			}
		}

		// this also the first encoded point
		// we save it in case we need to close the polygon later
		if firstEnc == nil {
			firstEnc = &mp[i]
		}

		// update what the last encoded value is
		lastEnc = &mp[i]

		switch count {
		case 0:
			err = enc.byte('(')
		default:
			err = enc.byte(',')
		}
		if err != nil {
			return err
		}

		count++
		err = enc.encodePair(v)
		if err != nil {
			return err
		}
	}

	// do size checking before encoding a closing point
	if count == 0 {
		return enc.string("EMPTY")
	}

	if (gType == polyType || gType == mPolyType) && count < 3 {
		return fmt.Errorf("not enough points for linear ring of POLYGON %v", mp)
	} else if (gType == lsType || gType == mlType) && count < 2 {
		return fmt.Errorf("not enough points for LINESTRING %v", mp)
	}

	// if we need to close the polygon/multipolygon
	// and the value we encoded last isn't (already) the last
	// value to encode
	if (gType == polyType || gType == mPolyType) && *firstEnc != *lastEnc {
		err = enc.byte(',')
		if err != nil {
			return err
		}
		err = enc.encodePair(*firstEnc)
		if err != nil {
			return err
		}
	}

	return enc.byte(')')
}

const (
	mpType byte = iota
	lsType
	mlType
	polyType
	mPolyType
)

func (enc Encoder) encodeLines(lines [][][2]float64, last int, gType byte) error {
	if gType != mlType {
		idx := lastNonEmptyIdxLines(lines)
		if idx != last && enc.strict {
			switch gType {
			case polyType:
				return errors.New("cannot have empty linear ring in strict POLYGON")
			case mPolyType:
				return errors.New("cannot have empty linear ring in strict MULTIPOLYGON")
			case mlType:
				// empty linestrings are allowed in multilines
				break
			default:
				panic("unreachable")
			}
		} else {
			last = idx
		}
	}

	if last == -1 {
		return enc.string("EMPTY")
	}

	err := enc.byte('(')
	if err != nil {
		return err
	}

	for _, v := range lines[:last] {
		// polygons and multipolygons cannot have empty linestrings
		if lastNonEmptyIdxPoints(v) == -1 {
			if enc.strict {
				switch gType {
				case polyType:
					return errors.New("cannot have empty linear ring in strict POLYGON")
				case mPolyType:
					return errors.New("cannot have empty linear ring in strict MULTIPOLYGON")
				case mlType:
					// empty linestrings are allowed in
					// encodePoints writes EMPTY
					break
				default:
					panic("unreachable")
				}
			} else {
				// empty linestrings are allowed in
				// encodePoints writes EMPTY
				if gType != mlType {
					continue
				}
			}
		}

		err := enc.encodePoints(v, len(v)-1, gType)
		if err != nil {
			return err
		}

		err = enc.byte(',')
		if err != nil {
			return err
		}
	}

	err = enc.encodePoints(lines[last], len(lines[last])-1, gType)
	if err != nil {
		return err
	}

	return enc.byte(')')
}

func (enc Encoder) encodePolys(polys [][][][2]float64, last int) error {
	if last == -1 {
		return enc.string("EMPTY")
	}

	err := enc.byte('(')
	if err != nil {
		return err
	}

	for _, v := range polys[:last] {
		err = enc.encodeLines(v, len(v)-1, mPolyType)
		if err != nil {
			return err
		}

		err = enc.byte(',')
		if err != nil {
			return err
		}
	}

	err = enc.encodeLines(polys[last], len(polys[last])-1, mPolyType)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	return enc.byte(')')
}

func (enc Encoder) encode(geo geom.Geometry) error {

	switch g := geo.(type) {
	case geom.Point:
		err := enc.string("POINT ")
		if err != nil {
			return err
		}

		return enc.encodePoint(g.XY())

	case *geom.Point:
		err := enc.string("POINT ")
		if err != nil {
			return err
		}

		if g == nil {
			return enc.string("EMPTY")
		}

		return enc.encode(g.XY())

	case geom.MultiPoint:
		err := enc.string("MULTIPOINT ")
		if err != nil {
			return err
		}

		return enc.encodePoints(g.Points(), len(g)-1, mpType)

	case *geom.MultiPoint:
		err := enc.string("MULTIPOINT ")
		if err != nil {
			return err
		}

		if g == nil {
			return enc.string("EMPTY")
		}

		return enc.encodePoints(g.Points(), len(*g)-1, mpType)

	case geom.LineString:
		err := enc.string("LINESTRING ")
		if err != nil {
			return err
		}

		return enc.encodePoints(g, len(g)-1, lsType)

	case *geom.LineString:
		err := enc.string("LINESTRING ")
		if err != nil {
			return err
		}

		if g == nil {
			return enc.string("EMPTY")
		}

		return enc.encodePoints(g.Vertices(), len(*g)-1, lsType)

	case geom.MultiLineString:
		err := enc.string("MULTILINESTRING ")
		if err != nil {
			return err
		}

		return enc.encodeLines(g.LineStrings(), len(g)-1, mlType)

	case *geom.MultiLineString:
		err := enc.string("MULTILINESTRING ")
		if err != nil {
			return err
		}

		if g == nil {
			return enc.string("EMPTY")
		}

		return enc.encodeLines(g.LineStrings(), len(*g)-1, mlType)

	case geom.Polygon:
		err := enc.string("POLYGON ")
		if err != nil {
			return err
		}

		return enc.encodeLines(g.LinearRings(), len(g)-1, polyType)

	case *geom.Polygon:
		err := enc.string("POLYGON ")
		if err != nil {
			return err
		}

		if g == nil {
			return enc.string("EMPTY")
		}

		return enc.encodeLines(g.LinearRings(), len(*g)-1, polyType)

	case geom.MultiPolygon:
		err := enc.string("MULTIPOLYGON ")
		if err != nil {
			return err
		}

		return enc.encodePolys(g, len(g)-1)

	case *geom.MultiPolygon:
		err := enc.string("MULTIPOLYGON ")
		if err != nil {
			return err
		}

		if g == nil {
			return enc.string("EMPTY")
		}

		return enc.encodePolys(g.Polygons(), len(*g)-1)

	case geom.Collection:
		if len(g) == 0 {
			return enc.string("GEOMETRYCOLLECTION EMPTY")
		}

		err := enc.string("GEOMETRYCOLLECTION ")
		if err != nil {
			return err
		}

		err = enc.byte('(')
		if err != nil {
			return err
		}

		last := len(g) - 1

		for _, v := range g[:last] {
			err = enc.encode(v)
			if err != nil {
				return err
			}

			err = enc.byte(',')
			if err != nil {
				return err
			}
		}

		err = enc.encode(g[last])
		if err != nil {
			return err
		}

		return enc.byte(')')

	case *geom.Collection:
		if g == nil {
			return enc.string("GEOMETRYCOLLECTION EMPTY")
		}

		return enc.encode(*g)

	// non basic types

	case [2]float64:
		return enc.encode(geom.Point(g))

	case [][2]float64:
		return enc.encode(geom.MultiPoint(g))

	case [][][2]float64:
		return enc.encode(geom.MultiLineString(g))

	case geom.Line:
		return enc.encode(geom.LineString(g[:]))

	case [2][2]float64:
		return enc.encode(geom.LineString(g[:]))

	case []geom.Line:
		lines := make(geom.MultiLineString, len(g))
		for i := range g {
			lines[i] = [][2]float64(g[i][:])
		}

		return enc.encode(lines)

	case []geom.Point:
		points := make(geom.MultiPoint, len(g))
		for i, v := range g {
			points[i] = v
		}

		return enc.encode(points)

	case geom.Triangle:
		return enc.encode(geom.Polygon{g[:]})

	case []geom.Triangle:
		mp := make([][][][2]float64, len(g))
		for i := range g {
			mp[i] = [][][2]float64{g[i][:]}
		}

		return enc.encode(geom.MultiPolygon(mp))

	case geom.Extent:
		return enc.encode(g.AsPolygon())

	case *geom.Extent:
		if g != nil {
			return enc.encode(g.AsPolygon())
		}

		return enc.encode(geom.Polygon{})

	default:
		return fmt.Errorf("unknown geometry: %T", geo)
	}
}

// Encode traverses the geometry and writes its WKT representation to the
// encoder's io.Writer and returns the first error it may have gotten.
func (enc Encoder) Encode(geo geom.Geometry) error {
	return enc.encode(geo)
}

// NewEncoder will clone the attributes of the current Encoder and swap out the writer
func (enc Encoder) NewEncoder(w io.Writer) Encoder {
	enc.w = w
	return enc
}

// EncodeString is like Encode except it will return a string instead of encode to the internal io.Writer
func (enc Encoder) EncodeString(geo geom.Geometry) (string, error) {
	var str strings.Builder
	if err := enc.NewEncoder(&str).encode(geo); err != nil {
		return "", err
	}
	return str.String(), nil
}

// MustEncode is like Encode except it will panic if there is an error
func (enc Encoder) MustEncode(geo geom.Geometry) string {
	var str strings.Builder
	e := enc.NewEncoder(&str)
	err := e.encode(geo)
	if err != nil {
		panic(err)
	}
	return str.String()
}
