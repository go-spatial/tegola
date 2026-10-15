package geometrycodec

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/alexeydott/geom"
)

const rawMaxDepth = 32
const rawMaxElements = 1 << 20
const rawMaxInputBytes = 64 << 20

// DecodeRawWKB decodes strict base XY or ISO-WKB Z (1001-1007), independently
// of the legacy tile decoder. EWKB and M/ZM are unsupported. A point whose
// complete coordinate tuple is NaN is empty (nil); partial NaN/Inf is corrupt.
// Empty point members of MultiPoint are unsupported; whole empty families
// remain typed empty. Collection members may retain nil empty points.
func DecodeRawWKB(v any) (geom.Geometry, error) {
	if v == nil {
		return nil, nil
	}
	data, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("raw WKB requires bytes, got %T", v)
	}
	if len(data) > rawMaxInputBytes {
		return nil, fmt.Errorf("raw WKB input exceeds byte limit")
	}
	decoder := rawWKBDecoder{data: data}
	decoded, err := decoder.geometry(0)
	if err != nil {
		return nil, fmt.Errorf("raw WKB: %w", err)
	}
	if decoder.offset != len(data) {
		return nil, fmt.Errorf("raw WKB trailing data")
	}
	return decoded.geometry, nil
}

type rawDecoded struct {
	geometry geom.Geometry
	kind     uint32
	z        bool
}
type rawWKBDecoder struct {
	data     []byte
	offset   int
	elements uint64
}

func (d *rawWKBDecoder) take(n int) ([]byte, error) {
	if n < 0 || n > len(d.data)-d.offset {
		return nil, fmt.Errorf("truncated input")
	}
	bytes := d.data[d.offset : d.offset+n]
	d.offset += n
	return bytes, nil
}
func (d *rawWKBDecoder) count(order binary.ByteOrder, minBytes uint64) (int, error) {
	bytes, err := d.take(4)
	if err != nil {
		return 0, err
	}
	count := uint64(order.Uint32(bytes))
	if count > rawMaxElements-d.elements || count*minBytes > uint64(len(d.data)-d.offset) {
		return 0, fmt.Errorf("element count exceeds input or budget")
	}
	d.elements += count
	return int(count), nil
}
func (d *rawWKBDecoder) position(order binary.ByteOrder, z, emptyPoint bool) ([3]float64, bool, error) {
	dimensions := 2
	if z {
		dimensions = 3
	}
	bytes, err := d.take(dimensions * 8)
	if err != nil {
		return [3]float64{}, false, err
	}
	var position [3]float64
	allNaN := true
	for axis := range dimensions {
		position[axis] = math.Float64frombits(order.Uint64(bytes[axis*8:]))
		allNaN = allNaN && math.IsNaN(position[axis])
	}
	if emptyPoint && allNaN {
		return position, true, nil
	}
	for axis := range dimensions {
		if math.IsNaN(position[axis]) || math.IsInf(position[axis], 0) {
			return position, false, fmt.Errorf("nonfinite position")
		}
	}
	return position, false, nil
}
func (d *rawWKBDecoder) positions(order binary.ByteOrder, z bool) ([][3]float64, error) {
	dimensions := 2
	if z {
		dimensions = 3
	}
	count, err := d.count(order, uint64(dimensions*8))
	if err != nil {
		return nil, err
	}
	points := make([][3]float64, count)
	for i := range points {
		value, _, err := d.position(order, z, false)
		if err != nil {
			return nil, err
		}
		points[i] = value
	}
	return points, nil
}
func (d *rawWKBDecoder) geometry(depth int) (rawDecoded, error) {
	if depth > rawMaxDepth {
		return rawDecoded{}, fmt.Errorf("maximum nesting depth exceeded")
	}
	header, err := d.take(5)
	if err != nil {
		return rawDecoded{}, err
	}
	var order binary.ByteOrder
	switch header[0] {
	case 0:
		order = binary.BigEndian
	case 1:
		order = binary.LittleEndian
	default:
		return rawDecoded{}, fmt.Errorf("invalid byte order")
	}
	code := order.Uint32(header[1:])
	z := code >= 1001 && code <= 1007
	kind := code
	if z {
		kind -= 1000
	}
	if kind < 1 || kind > 7 {
		return rawDecoded{}, fmt.Errorf("%w: geometry type %d (M/ZM/EWKB excluded)", ErrUnsupportedRawGeometry, code)
	}
	decoded := rawDecoded{kind: kind, z: z}
	switch kind {
	case 1:
		position, empty, err := d.position(order, z, true)
		if err != nil {
			return decoded, err
		}
		if !empty {
			if z {
				decoded.geometry = geom.PointZ(position)
			} else {
				decoded.geometry = geom.Point{position[0], position[1]}
			}
		}
	case 2:
		points, err := d.positions(order, z)
		if err != nil {
			return decoded, err
		}
		if len(points) == 1 {
			return decoded, fmt.Errorf("line requires zero or at least two positions")
		}
		decoded.geometry = rawLine(points, z)
	case 3:
		count, err := d.count(order, 4)
		if err != nil {
			return decoded, err
		}
		rings := make([][][3]float64, count)
		for i := range rings {
			ring, err := d.positions(order, z)
			if err != nil {
				return decoded, err
			}
			if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
				return decoded, fmt.Errorf("polygon wire ring requires at least four positions and exact closure")
			}
			rings[i] = ring
		}
		decoded.geometry = rawPolygon(rings, z)
	default:
		count, err := d.count(order, 5)
		if err != nil {
			return decoded, err
		}
		children := make([]rawDecoded, count)
		for i := range children {
			child, err := d.geometry(depth + 1)
			if err != nil {
				return decoded, err
			}
			if kind != 7 && (child.kind != kind-3 || child.z != z) {
				return decoded, fmt.Errorf("multi-geometry member type or dimension mismatch")
			}
			children[i] = child
		}
		decoded.geometry, err = rawMulti(kind, z, children)
		if err != nil {
			return decoded, err
		}
	}
	return decoded, nil
}
func rawLine(points [][3]float64, z bool) geom.Geometry {
	if z {
		return geom.LineStringZ(points)
	}
	values := make(geom.LineString, len(points))
	for i, p := range points {
		values[i] = [2]float64{p[0], p[1]}
	}
	return values
}
func rawPolygon(rings [][][3]float64, z bool) geom.Geometry {
	if z {
		return geom.PolygonZ(rings)
	}
	values := make(geom.Polygon, len(rings))
	for i, ring := range rings {
		values[i] = rawLine(ring, false).(geom.LineString)
	}
	return values
}
func rawMulti(kind uint32, z bool, children []rawDecoded) (geom.Geometry, error) {
	switch kind {
	case 4:
		if z {
			values := make(geom.MultiPointZ, len(children))
			for i, child := range children {
				if child.geometry == nil {
					return nil, fmt.Errorf("%w: empty point member of MultiPoint", ErrUnsupportedRawGeometry)
				}
				values[i] = child.geometry.(geom.PointZ)
			}
			return values, nil
		}
		values := make(geom.MultiPoint, len(children))
		for i, child := range children {
			if child.geometry == nil {
				return nil, fmt.Errorf("%w: empty point member of MultiPoint", ErrUnsupportedRawGeometry)
			}
			values[i] = child.geometry.(geom.Point)
		}
		return values, nil
	case 5:
		if z {
			values := make(geom.MultiLineStringZ, len(children))
			for i, child := range children {
				values[i] = child.geometry.(geom.LineStringZ)
			}
			return values, nil
		}
		values := make(geom.MultiLineString, len(children))
		for i, child := range children {
			values[i] = child.geometry.(geom.LineString)
		}
		return values, nil
	case 6:
		if z {
			values := make(MultiPolygonZ, len(children))
			for i, child := range children {
				values[i] = child.geometry.(geom.PolygonZ)
			}
			return values, nil
		}
		values := make(geom.MultiPolygon, len(children))
		for i, child := range children {
			values[i] = child.geometry.(geom.Polygon)
		}
		return values, nil
	case 7:
		values := make(geom.Collection, len(children))
		for i, child := range children {
			values[i] = child.geometry
		}
		return values, nil
	}
	return nil, fmt.Errorf("%w: multi-geometry", ErrUnsupportedRawGeometry)
}
