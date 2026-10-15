package decode

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb/internal/consts"
)

type ErrBadBOM byte

func (e ErrBadBOM) Error() string { return "decode: bad byte order marker" }

type ErrInvalidType struct {
	// In which collection type was the invalide type found.
	Primary string
	Type    uint32
}

func (e ErrInvalidType) Error() string {
	return fmt.Sprintf("decode: invalid type for %v", e.Primary)
}

// ErrElementCount is returned when a WKB header declares an element or point
// count whose minimum encoded size exceeds the remaining input. Checking the
// count against the remaining bytes before allocating stops hostile or corrupt
// inputs from driving huge allocations (OOM) for data that could never decode.
type ErrElementCount struct {
	Primary string
	Count   uint32
}

func (e ErrElementCount) Error() string {
	return fmt.Sprintf("decode: %v element count %v exceeds input size", e.Primary, e.Count)
}

// MaxNestingDepth is the maximum number of nested GeometryCollection levels
// the decoder will follow. Each level costs one recursion frame (and one
// decoder call per element), so hostile input with unbounded nesting could
// otherwise exhaust the goroutine stack. A collection nested deeper than this
// is rejected with ErrMaxNestingDepth.
const MaxNestingDepth = 32

// ErrMaxNestingDepth is returned when GeometryCollections are nested more
// deeply than MaxNestingDepth levels.
type ErrMaxNestingDepth struct {
	Max int
}

func (e ErrMaxNestingDepth) Error() string {
	return fmt.Sprintf("decode: maximum nesting depth of %d nested collections exceeded", e.Max)
}

// MaxElements is the absolute maximum number of elements (or points) a single
// WKB header may declare when the input reader cannot report its remaining
// length. For such readers the remaining-size pre-check cannot run, so this
// cap is what stops a hostile input from forcing a huge allocation. Readers
// that report their remaining length (e.g. the *bytes.Reader behind
// wkb.DecodeBytes) are instead checked against the actual input size and may
// legitimately contain more elements than this.
const MaxElements = 1 << 20

// ensureCount verifies that num elements of at least minBytesPerElem each can
// fit into the remaining bytes of r, before any slice is allocated for them.
// Readers that report their remaining length (e.g. the *bytes.Reader behind
// wkb.DecodeBytes) are checked against it; opaque io.Readers cannot be, so
// their declared counts are capped at MaxElements instead.
func ensureCount(r io.Reader, primary string, num uint32, minBytesPerElem uint64) error {
	if lr, ok := r.(interface{ Len() int }); ok {
		if uint64(num)*minBytesPerElem > uint64(lr.Len()) {
			return ErrElementCount{Primary: primary, Count: num}
		}
		return nil
	}
	if uint64(num) > MaxElements {
		return ErrElementCount{Primary: primary, Count: num}
	}
	return nil
}

func ByteOrderType(r io.Reader) (byteOrder binary.ByteOrder, typ uint32, err error) {
	var bom = make([]byte, 1, 1)
	// the bom is the first byte
	if _, err = r.Read(bom); err != nil {
		return byteOrder, typ, err
	}

	// the bom should be either 0 or 1
	switch bom[0] {
	case 0:
		byteOrder = binary.BigEndian
	case 1:
		byteOrder = binary.LittleEndian
	default:
		return byteOrder, typ, ErrBadBOM(bom[0])
	}

	// Reading the type which is 4 bytes
	err = binary.Read(r, byteOrder, &typ)
	return byteOrder, typ, err
}

func Point(r io.Reader, bom binary.ByteOrder) (pt geom.Point, err error) {
	err = binary.Read(r, bom, &pt)
	return pt, err
}
func MultiPoint(r io.Reader, bom binary.ByteOrder) (pts geom.MultiPoint, err error) {
	var num, typ uint32 // Number of points
	err = binary.Read(r, bom, &num)
	if err != nil {
		return pts, err
	}
	// Each point needs at least a byte order marker, a type and two
	// coordinates (5 + 16 bytes) in the input.
	if err = ensureCount(r, "multipoint", num, 21); err != nil {
		return pts, err
	}

	pts = make([][2]float64, num)
	for i := range pts {

		bom, typ, err = ByteOrderType(r)
		if err != nil {
			return pts, err
		}
		if typ != consts.Point {
			return pts, ErrInvalidType{"multipoint", typ}
		}
		err = binary.Read(r, bom, &pts[i])
		if err != nil {
			return pts, err
		}
	}
	return pts, err
}

func LineString(r io.Reader, bom binary.ByteOrder) (ln geom.LineString, err error) {
	var num uint32 // Number of points
	if err = binary.Read(r, bom, &num); err != nil {
		return ln, err
	}
	// Each point needs at least two float64 coordinates (16 bytes).
	if err = ensureCount(r, "linestring", num, 16); err != nil {
		return ln, err
	}
	ln = make([][2]float64, num)
	for i := range ln {
		if err = binary.Read(r, bom, &ln[i]); err != nil {
			return ln, err
		}
	}
	return ln, err
}

func MultiLineString(r io.Reader, bom binary.ByteOrder) (lns geom.MultiLineString, err error) {
	var num uint32
	if err = binary.Read(r, bom, &num); err != nil {
		return lns, err
	}
	// Each line needs at least a byte order marker, a type and a point
	// count (5 + 4 bytes) in the input.
	if err = ensureCount(r, "multilinestring", num, 9); err != nil {
		return lns, err
	}
	lns = make([][][2]float64, num)
	for i := range lns {
		bom, typ, err := ByteOrderType(r)
		if err != nil {
			return lns, err
		}
		if typ != consts.LineString {
			return lns, ErrInvalidType{"multilinestring", typ}
		}
		if lns[i], err = LineString(r, bom); err != nil {
			return lns, err
		}
	}
	return lns, err
}

func LinerRing(r io.Reader, bom binary.ByteOrder) (rn [][2]float64, err error) {
	var num uint32 // Number of points
	if err = binary.Read(r, bom, &num); err != nil {
		return rn, err
	}
	// Each point needs at least two float64 coordinates (16 bytes).
	if err = ensureCount(r, "ring", num, 16); err != nil {
		return rn, err
	}
	rn = make([][2]float64, num)
	for i := range rn {
		if err = binary.Read(r, bom, &rn[i]); err != nil {
			return rn, err
		}
	}
	if num > 1 {
		// Remove the last point if it is the same.
		if rn[0][0] == rn[num-1][0] && rn[0][1] == rn[num-1][1] {
			rn = rn[:num-1]
		}
	}

	return rn, err
}

func Polygon(r io.Reader, bom binary.ByteOrder) (ply geom.Polygon, err error) {
	var num uint32
	if err = binary.Read(r, bom, &num); err != nil {
		return ply, err
	}
	// Each ring needs at least its point count (4 bytes) in the input.
	if err = ensureCount(r, "polygon", num, 4); err != nil {
		return ply, err
	}
	ply = make([][][2]float64, num)
	for i := range ply {
		if ply[i], err = LinerRing(r, bom); err != nil {
			return ply, err
		}
	}
	return ply, err
}

func MultiPolygon(r io.Reader, bom binary.ByteOrder) (plys geom.MultiPolygon, err error) {
	var num uint32
	if err = binary.Read(r, bom, &num); err != nil {
		return plys, err
	}
	// Each polygon needs at least a byte order marker, a type and a ring
	// count (5 + 4 bytes) in the input.
	if err = ensureCount(r, "multipolygon", num, 9); err != nil {
		return plys, err
	}
	plys = make([][][][2]float64, num)
	for i := range plys {
		bom, typ, err := ByteOrderType(r)
		if err != nil {
			return plys, err
		}
		if typ != consts.Polygon {
			return plys, ErrInvalidType{"multipolygon", typ}
		}
		if plys[i], err = Polygon(r, bom); err != nil {
			return plys, err
		}
	}
	return plys, err
}

// Collection decodes a WKB GeometryCollection. Nested collections are limited
// to MaxNestingDepth levels; deeper nesting returns ErrMaxNestingDepth instead
// of recursing until the goroutine stack is exhausted (audit P5-4).
func Collection(r io.Reader, bom binary.ByteOrder) (col geom.Collection, err error) {
	return collection(r, bom, 1)
}

func collection(r io.Reader, bom binary.ByteOrder, depth int) (col geom.Collection, err error) {
	var num uint32
	if err = binary.Read(r, bom, &num); err != nil {
		return col, err
	}
	// Each element needs at least a byte order marker and a type (5 bytes)
	// in the input.
	if err = ensureCount(r, "collection", num, 5); err != nil {
		return col, err
	}
	col = make(geom.Collection, num)
	for i := range col {
		bom, typ, err := ByteOrderType(r)
		if err != nil {
			return col, err
		}
		switch typ {
		case consts.Point:
			col[i], err = Point(r, bom)
		case consts.LineString:
			col[i], err = LineString(r, bom)
		case consts.Polygon:
			col[i], err = Polygon(r, bom)
		case consts.MultiPoint:
			col[i], err = MultiPoint(r, bom)
		case consts.MultiLineString:
			col[i], err = MultiLineString(r, bom)
		case consts.MultiPolygon:
			col[i], err = MultiPolygon(r, bom)
		case consts.Collection:
			if depth+1 > MaxNestingDepth {
				err = ErrMaxNestingDepth{Max: MaxNestingDepth}
			} else {
				col[i], err = collection(r, bom, depth+1)
			}
		default:
			err = ErrInvalidType{"collection", typ}
		}
		if err != nil {
			return col, err
		}
	}
	return col, err
}
