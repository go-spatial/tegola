package querytest

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

// EncodeFixtureWKB writes declared literal fixture coordinates as ISO WKB.
// It never decodes geometry or computes query expectations. Nil belongs in SQL
// NULL and is rejected here; nonfinite literals may intentionally test bad wire.
func EncodeFixtureWKB(g geom.Geometry) ([]byte, error) {
	return encodeFixtureWKB(g, 1)
}

func encodeFixtureWKB(g geom.Geometry, depth int) ([]byte, error) {
	if depth > 32 || g == nil {
		return nil, fmt.Errorf("querytest: unsupported fixture geometry or depth")
	}
	var code uint32
	var payload []byte
	putPositions := func(positions [][]float64) {
		for _, position := range positions {
			for _, ordinate := range position {
				payload = binary.LittleEndian.AppendUint64(payload, math.Float64bits(ordinate))
			}
		}
	}
	putXY := func(points [][2]float64) {
		payload = binary.LittleEndian.AppendUint32(payload, uint32(len(points)))
		for _, p := range points {
			putPositions([][]float64{p[:]})
		}
	}
	putXYZ := func(points [][3]float64) {
		payload = binary.LittleEndian.AppendUint32(payload, uint32(len(points)))
		for _, p := range points {
			putPositions([][]float64{p[:]})
		}
	}
	putChildren := func(children []geom.Geometry) error {
		payload = binary.LittleEndian.AppendUint32(payload, uint32(len(children)))
		for _, child := range children {
			bytes, err := encodeFixtureWKB(child, depth+1)
			if err != nil {
				return err
			}
			payload = append(payload, bytes...)
		}
		return nil
	}
	var err error
	switch v := g.(type) {
	case geom.Point:
		code = 1
		putPositions([][]float64{v[:]})
	case geom.PointZ:
		code = 1001
		putPositions([][]float64{v[:]})
	case geom.LineString:
		code = 2
		putXY(v)
	case geom.LineStringZ:
		code = 1002
		putXYZ(v)
	case geom.Polygon:
		code = 3
		payload = binary.LittleEndian.AppendUint32(payload, uint32(len(v)))
		for _, ring := range v {
			putXY(ring)
		}
	case geom.PolygonZ:
		code = 1003
		payload = binary.LittleEndian.AppendUint32(payload, uint32(len(v)))
		for _, ring := range v {
			putXYZ(ring)
		}
	case geom.MultiPoint:
		code = 4
		children := make([]geom.Geometry, len(v))
		for i, p := range v {
			children[i] = geom.Point(p)
		}
		err = putChildren(children)
	case geom.MultiPointZ:
		code = 1004
		children := make([]geom.Geometry, len(v))
		for i, p := range v {
			children[i] = geom.PointZ(p)
		}
		err = putChildren(children)
	case geom.MultiLineString:
		code = 5
		children := make([]geom.Geometry, len(v))
		for i, line := range v {
			children[i] = geom.LineString(line)
		}
		err = putChildren(children)
	case geom.MultiLineStringZ:
		code = 1005
		children := make([]geom.Geometry, len(v))
		for i, line := range v {
			children[i] = geom.LineStringZ(line)
		}
		err = putChildren(children)
	case geom.MultiPolygon:
		code = 6
		children := make([]geom.Geometry, len(v))
		for i, polygon := range v {
			children[i] = geom.Polygon(polygon)
		}
		err = putChildren(children)
	case geometrycodec.MultiPolygonZ:
		code = 1006
		children := make([]geom.Geometry, len(v))
		for i, polygon := range v {
			children[i] = geom.PolygonZ(polygon)
		}
		err = putChildren(children)
	case geom.Collection:
		code = 7
		err = putChildren(v)
	default:
		return nil, fmt.Errorf("querytest: unsupported fixture geometry family")
	}
	if err != nil {
		return nil, err
	}
	header := []byte{1}
	header = binary.LittleEndian.AppendUint32(header, code)
	return append(header, payload...), nil
}
