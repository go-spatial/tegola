// Package mos implements direct decoding of MapplGIS MOS geometry blobs
// (the proprietary binary format serialized by TMapObjectStructureBase,
// MapplGIS blob reader layout).
//
// Format (all values little-endian, coordinates are quantized int32 pairs):
//
//	Header (12 bytes, the packed Delphi THeaderObject as written by
//	TMapObjectStructureBase.PutToBufInternal):
//	  0: oType             byte   (0=polygon, 1=polyline, 2=point, 3=text, 4=image)
//	  1: oTypeModification byte
//	  2: AddFlag           uint16  (0)
//	  4: subObjectsCount   uint16
//	  6: pointsCount       int32  (total across all subobjects)
//	 10: ofl               uint16  (object flags)
//
//	Then subObjectsCount x uint32 point counts (one per subobject).
//	Then pointsCount x (int32 x, int32 y) — all subobjects' points
//	contiguously, in subobject order.
//
//	A 10-byte prefix (without the trailing ofl word) is also accepted on
//	decode for older Tegola fixtures, but the native MapplGIS layout is the
//	12-byte header above.
//
//	Everything after the points block (point icon params, labels, markers,
//	multi-label texts, bezier control points) is non-geometric or optional
//	detail and is ignored by this package; the geometry prefix is fully
//	self-describing.
//
// Real coordinates are recovered as x / kPrecision + OffsetX where
// kPrecision = 10^Precision. Precision is the number of decimal digits the
// quantized integers carry (e.g. 3 means millimetre precision for metre
// units, matching MapplGIS layer SystemInfo.kPrecision).
//
// Point handling mirrors GetFromBufInternal: consecutive duplicate points
// are dropped within each subobject, and for polygons a final vertex equal
// to the first one is dropped (rings are re-closed when building geometry).
package mos

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/alexeydott/geom"
)

// MOS object types (THeaderObject.oType).
const (
	TypePolygon  = 0
	TypePolyline = 1
	TypePoint    = 2
	TypeText     = 3
	TypeImage    = 4
)

// Object flag bits (THeaderObject.ofl). Declared for completeness of the
// format description; the geometry prefix does not depend on them.
const (
	FlagFixLabel   uint16 = 1 << 0
	FlagMultiLabel uint16 = 1 << 1
	FlagFixLabelID uint16 = 1 << 2
	FlagFixMarkers uint16 = 1 << 3
	FlagBezier     uint16 = 1 << 4
)

const (
	// headerSize is the legacy short form without the trailing ofl word:
	// 1 + 1 + 2 + 2 + 4.
	headerSize = 10
	// extendedHeaderSize is the native packed THeaderObject written by
	// TMapObjectStructureBase.PutToBufInternal: 1 + 1 + 2 + 2 + 4 + 2.
	extendedHeaderSize = 12
)

// Header mirrors the packed THeaderObject geometry prefix. Flags is populated
// when the optional 12-byte extension is present.
type Header struct {
	ObjectType       byte
	TypeModification byte
	AddFlag          uint16
	SubObjectsCount  int
	PointsCount      int
	Flags            uint16
}

// Options controls decoding of a MOS blob.
type Options struct {
	// Precision is the number of decimal digits the quantized integer
	// coordinates carry (e.g. 3 = millimetres for metre units). Stored
	// values are divided by 10^Precision. Defaults to 0 (integer units).
	Precision float64
	// OffsetX/OffsetY are added to the dequantized coordinates. MapplGIS
	// stores blobs with a zero offset, defaults to 0.
	OffsetX, OffsetY float64
	// UnitFactor scales the dequantized coordinates after applying
	// Precision and the offsets. It converts the layer's map units
	// (from MapplGIS LayerInfo map units) to metres of the projected
	// CRS, e.g. 0.001 for millimetre units. Defaults to 0 (treated as 1).
	UnitFactor float64
}

// unitFactor returns the configured unit scaling factor, defaulting to 1.
func (o Options) unitFactor() float64 {
	if o.UnitFactor == 0 {
		return 1
	}
	return o.UnitFactor
}

// kPrecision converts the configured decimal precision into the multiplicative
// precision factor used by MapplGIS (x / kPrecision + OffsetX).
// Delphi GetKPrecision caps precision at MAX_PRECISION=10:
// Result := IntPower(10, Max(MIN(aPrecision, MAX_PRECISION), 0)).
func (o Options) kPrecision() (float64, error) {
	if math.IsNaN(o.Precision) || math.IsInf(o.Precision, 0) ||
		math.Trunc(o.Precision) != o.Precision {
		return 0, fmt.Errorf("mos: invalid precision %v", o.Precision)
	}
	p := o.Precision
	if p < 0 {
		p = 0
	}
	if p > 10 {
		p = 10
	}
	return math.Pow(10, p), nil
}

// DecodeHeader parses the native MOS blob header. Both the native 10-byte
// prefix and the legacy 12-byte flags extension are accepted.
func DecodeHeader(buf []byte) (Header, error) {
	if len(buf) < headerSize {
		return Header{}, fmt.Errorf("mos: buffer too short (%v bytes) for MOS header", len(buf))
	}
	if len(buf) == headerSize {
		return parseHeader(buf, headerSize)
	}
	if len(buf) == extendedHeaderSize {
		return parseHeader(buf, extendedHeaderSize)
	}
	h, _, err := decodeHeaderAndOffset(buf)
	return h, err
}

// Decode decodes the geometry prefix of a MOS blob into a tegola geometry.
//
// Mapping of MOS object types to geometries:
//   - polygon: each subobject is a ring; containment classifies exterior
//     rings and holes into geom.Polygon or geom.MultiPolygon.
//   - polyline: one subobject -> geom.LineString, several -> geom.MultiLineString.
//   - point: one point -> geom.Point, several -> geom.MultiPoint.
//   - text / image: anchor point of the first subobject -> geom.Point.
func Decode(buf []byte, opts Options) (geom.Geometry, error) {
	h, geometryOffset, err := decodeHeaderAndOffset(buf)
	if err != nil {
		return nil, err
	}
	k, err := opts.kPrecision()
	if err != nil {
		return nil, err
	}

	c := &cursor{b: buf, pos: geometryOffset}

	// per-subobject point counts
	counts := make([]int, h.SubObjectsCount)
	total := 0
	for i := range counts {
		v, err := c.u32()
		if err != nil {
			return nil, fmt.Errorf("mos: reading point count of subobject %v: %v", i, err)
		}
		if v > math.MaxInt32-uint32(total) {
			return nil, fmt.Errorf("mos: subobject %v point count %v overflows", i, v)
		}
		counts[i] = int(v)
		total += counts[i]
	}
	if total != h.PointsCount {
		return nil, fmt.Errorf("mos: subobject point counts sum to %v but header declares %v", total, h.PointsCount)
	}
	if h.PointsCount > c.remaining()/8 {
		return nil, fmt.Errorf("mos: buffer holds %v points but header declares %v", c.remaining()/8, h.PointsCount)
	}

	// points, deduplicated per subobject (mirrors GetFromBufInternal with
	// flLoadEqualPoints=false)
	subObjects := make([][][2]float64, h.SubObjectsCount)
	var x0, y0 float64
	for i, count := range counts {
		pts := make([][2]float64, 0, count)
		for j := 0; j < count; j++ {
			xi, err := c.i32()
			if err != nil {
				return nil, fmt.Errorf("mos: reading point %v of subobject %v: %v", j, i, err)
			}
			yi, err := c.i32()
			if err != nil {
				return nil, fmt.Errorf("mos: reading point %v of subobject %v: %v", j, i, err)
			}
			x := (float64(xi)/k + opts.OffsetX) * opts.unitFactor()
			y := (float64(yi)/k + opts.OffsetY) * opts.unitFactor()
			// keep the first point of every subobject, drop consecutive
			// duplicates afterwards
			if j > 0 && x == x0 && y == y0 {
				continue
			}
			x0, y0 = x, y
			pts = append(pts, [2]float64{x, y})
		}
		// polygons: drop a final vertex equal to the first one; the ring is
		// closed again when building the geometry
		if h.ObjectType == TypePolygon && len(pts) > 1 && pts[0] == pts[len(pts)-1] {
			pts = pts[:len(pts)-1]
		}
		subObjects[i] = pts
	}

	return h.buildGeometry(subObjects)
}

// decodeHeaderAndOffset parses the common header fields and selects the
// layout whose subobject/point prefix is internally consistent. Native MapplGIS
// blobs use a 10-byte header; a 12-byte flags extension is retained for
// compatibility with older Tegola-generated fixtures.
func decodeHeaderAndOffset(buf []byte) (Header, int, error) {
	if len(buf) < headerSize {
		return Header{}, 0, fmt.Errorf("mos: buffer too short (%v bytes) for MOS header", len(buf))
	}

	base, err := parseHeader(buf, extendedHeaderSize)
	if err != nil {
		return Header{}, 0, err
	}

	// Prefer the native 12-byte form (packed THeaderObject as written by
	// TMapObjectStructureBase.PutToBufInternal). For a native 12-byte blob,
	// the bytes at offset 10 are the ofl flags word; treating them as the
	// first subobject count of a 10-byte blob misclassifies the layout, so
	// the 10-byte form is only a fallback for older fixtures.
	nativeErr := validateGeometryPrefix(buf, base, extendedHeaderSize)
	if nativeErr == nil {
		return base, extendedHeaderSize, nil
	}

	// Older Tegola fixtures used the short 10-byte prefix without ofl.
	var shortErr error
	if len(buf) >= headerSize {
		short, parseErr := parseHeader(buf, headerSize)
		if parseErr == nil {
			shortErr = validateGeometryPrefix(buf, short, headerSize)
			if shortErr == nil {
				return short, headerSize, nil
			}
		} else {
			shortErr = parseErr
		}
	}

	if shortErr != nil {
		return Header{}, 0, fmt.Errorf("mos: invalid geometry prefix: native: %v; short: %v", nativeErr, shortErr)
	}
	return Header{}, 0, fmt.Errorf("mos: invalid geometry prefix: %v", nativeErr)
}

func parseHeader(buf []byte, size int) (Header, error) {
	if len(buf) < size {
		return Header{}, fmt.Errorf("mos: buffer too short (%v bytes) for %v-byte MOS header", len(buf), size)
	}
	h := Header{
		ObjectType:       buf[0],
		TypeModification: buf[1],
		AddFlag:          binary.LittleEndian.Uint16(buf[2:4]),
		SubObjectsCount:  int(binary.LittleEndian.Uint16(buf[4:6])),
		PointsCount:      int(int32(binary.LittleEndian.Uint32(buf[6:10]))),
	}
	if size == extendedHeaderSize {
		h.Flags = binary.LittleEndian.Uint16(buf[10:12])
	}
	if h.ObjectType > TypeImage {
		return Header{}, fmt.Errorf("mos: unsupported object type %v", h.ObjectType)
	}
	if h.PointsCount < 0 {
		return Header{}, fmt.Errorf("mos: negative points count %v", h.PointsCount)
	}
	return h, nil
}

// validateGeometryPrefix verifies the counts and point block at offset. It
// deliberately ignores the optional tail because MOS stores labels, icons
// and other attributes after the geometry prefix.
func validateGeometryPrefix(buf []byte, h Header, offset int) error {
	if h.SubObjectsCount > (len(buf)-offset)/4 {
		return fmt.Errorf("subobject count %v exceeds remaining buffer", h.SubObjectsCount)
	}
	c := &cursor{b: buf, pos: offset}
	total := 0
	for i := 0; i < h.SubObjectsCount; i++ {
		v, err := c.u32()
		if err != nil {
			return fmt.Errorf("reading point count of subobject %v: %v", i, err)
		}
		if v > math.MaxInt32-uint32(total) {
			return fmt.Errorf("subobject %v point count %v overflows", i, v)
		}
		total += int(v)
	}
	if total != h.PointsCount {
		return fmt.Errorf("subobject point counts sum to %v but header declares %v", total, h.PointsCount)
	}
	if h.PointsCount > c.remaining()/8 {
		return fmt.Errorf("buffer holds %v points but header declares %v", c.remaining()/8, h.PointsCount)
	}
	return nil
}

// polygonGeometry classifies closed rings into exteriors and holes and
// returns a geom.Polygon (single exterior) or geom.MultiPolygon (several
// exteriors, e.g. an island inside a lake hole), mirroring the semantic of
// MarkSubObjectsRelation + ConvertPolygonGeometryToCanonicalFormat.
func polygonGeometry(rings [][][2]float64) (geom.Geometry, error) {
	// inner[j] is the tightest ring containing ring j (smallest area), -1 if
	// none. Containment mirrors the vertex-inside-contour test used by
	// MarkSubObjectsRelation; nesting depth then decides the role by parity:
	// even depth (0, 2, ...) is an exterior ring, odd depth (1, 3, ...) a
	// hole — so an island inside a lake hole becomes its own exterior,
	// matching ConvertPolygonGeometryToCanonicalFormat (IslandInHole branch).
	inner := make([]int, len(rings))
	areas := make([]float64, len(rings))
	for i := range rings {
		areas[i] = ringArea(rings[i])
	}
	for j := range rings {
		inner[j] = -1
		for i := range rings {
			if i == j || !ringContainsRing(rings[i], rings[j]) {
				continue
			}
			if inner[j] == -1 || areas[i] < areas[inner[j]] {
				inner[j] = i
			}
		}
	}

	// nesting depth per ring (memoized walk up the containment chain)
	depth := make([]int, len(rings))
	for i := range depth {
		depth[i] = -1
	}
	var depthOf func(int) int
	depthOf = func(i int) int {
		if depth[i] >= 0 {
			return depth[i]
		}
		if inner[i] < 0 || inner[i] == i {
			depth[i] = 0
			return 0
		}
		depth[i] = 0 // guard against pathological cycles while recursing
		d := depthOf(inner[i]) + 1
		depth[i] = d
		return d
	}
	for i := range rings {
		depthOf(i)
	}

	// even depth starts a polygon, odd depth attaches as a hole to the
	// directly containing (even-depth) ring
	polygons := make(geom.MultiPolygon, 0)
	extIdx := make([]int, len(rings))
	for i := range rings {
		if depth[i]%2 != 0 {
			continue
		}
		extIdx[i] = len(polygons)
		polygons = append(polygons, geom.Polygon{rings[i]})
	}
	for i := range rings {
		if depth[i]%2 == 0 {
			continue
		}
		p := extIdx[inner[i]]
		polygons[p] = append(polygons[p], rings[i])
	}
	if len(polygons) == 0 {
		return nil, fmt.Errorf("mos: polygon object has no exterior rings")
	}
	if len(polygons) == 1 {
		return geom.Polygon(polygons[0]), nil
	}
	return polygons, nil
}

// ringArea is the absolute shoelace area of a closed ring.
func ringArea(ring [][2]float64) float64 {
	a := 0.0
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a += (ring[j][0] + ring[i][0]) * (ring[j][1] - ring[i][1])
	}
	return math.Abs(a) / 2
}

// ringContainsRing reports whether ring a strictly contains ring b, using
// vertex-inside-polygon with any-point-on-boundary counting as inside
// (mirrors CheckPointInPolygon usage in MarkSubObjectsRelation).
func ringContainsRing(a, b [][2]float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	// a's bbox must fully cover b's bbox
	ax := [2]float64{math.Inf(1), math.Inf(-1)}
	ay := [2]float64{math.Inf(1), math.Inf(-1)}
	for _, p := range a {
		ax[0] = math.Min(ax[0], p[0])
		ax[1] = math.Max(ax[1], p[0])
		ay[0] = math.Min(ay[0], p[1])
		ay[1] = math.Max(ay[1], p[1])
	}
	for _, p := range b {
		if p[0] < ax[0] || p[0] > ax[1] || p[1] < ay[0] || p[1] > ay[1] {
			return false
		}
	}
	for _, p := range b {
		if !pointInRing(a, p) {
			return false
		}
	}
	return true
}

// pointInRing is an even-odd ray-cast test. Points exactly on an edge
// count as inside.
func pointInRing(ring [][2]float64, pt [2]float64) bool {
	in := false
	n := len(ring)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := ring[j], ring[i]
		// on-segment check
		cross := (b[0]-a[0])*(pt[1]-a[1]) - (b[1]-a[1])*(pt[0]-a[0])
		if math.Abs(cross) < 1e-12 &&
			math.Min(a[0], b[0]) <= pt[0] && pt[0] <= math.Max(a[0], b[0]) &&
			math.Min(a[1], b[1]) <= pt[1] && pt[1] <= math.Max(a[1], b[1]) {
			return true
		}
		if (a[1] > pt[1]) != (b[1] > pt[1]) {
			xx := (b[0]-a[0])*(pt[1]-a[1])/(b[1]-a[1]) + a[0]
			if pt[0] < xx {
				in = !in
			}
		}
	}
	return in
}

// buildGeometry maps the decoded subobjects to a tegola geometry according
// to the MOS object type.
//
// The polygon mapping follows TMapObjectStructureBase.ConvertPolygonGeometry
// MapplGIS canonical format logic: subobjects are classified by point-
// in-polygon containment into exterior rings (InnerIndex < 0) and holes
// (InnerIndex = exterior ring). One exterior ring yields a Polygon, several
// exterior rings a MultiPolygon (e.g. an island inside a lake hole).
func (h Header) buildGeometry(subObjects [][][2]float64) (geom.Geometry, error) {
	switch h.ObjectType {
	case TypePolygon:
		var rings [][][2]float64
		for _, so := range subObjects {
			// mirrors ValidateSubObjectSpatialData: a valid contour holds at
			// least 3 points; after dedup a shorter one is degenerate
			if len(so) < 3 {
				continue
			}
			// close the ring
			ring := make([][2]float64, 0, len(so)+1)
			ring = append(ring, so...)
			ring = append(ring, so[0])
			rings = append(rings, ring)
		}
		if len(rings) == 0 {
			return nil, fmt.Errorf("mos: polygon object has no rings")
		}
		return polygonGeometry(rings)

	case TypePolyline:
		var lines [][][2]float64
		for _, so := range subObjects {
			// mirrors ValidateSubObjectSpatialData: a valid contour holds at
			// least 2 points
			if len(so) >= 2 {
				lines = append(lines, so)
			}
		}
		if len(lines) == 0 {
			return nil, fmt.Errorf("mos: polyline object has no lines")
		}
		if len(lines) == 1 {
			return geom.LineString(lines[0]), nil
		}
		return geom.MultiLineString(lines), nil

	case TypePoint:
		var pts [][2]float64
		for _, so := range subObjects {
			pts = append(pts, so...)
		}
		switch len(pts) {
		case 0:
			return nil, fmt.Errorf("mos: point object has no points")
		case 1:
			return geom.Point(pts[0]), nil
		default:
			return geom.MultiPoint(pts), nil
		}

	case TypeText, TypeImage:
		// text and image objects are anchored at the first vertex of the
		// first subobject
		for _, so := range subObjects {
			if len(so) > 0 {
				return geom.Point(so[0]), nil
			}
		}
		return nil, fmt.Errorf("mos: text/image object has no anchor point")

	default:
		return nil, fmt.Errorf("mos: unsupported object type %v", h.ObjectType)
	}
}

// cursor is a bounds-checked little-endian reader over a byte slice.
type cursor struct {
	b   []byte
	pos int
}

func (c *cursor) remaining() int { return len(c.b) - c.pos }

func (c *cursor) need(n int) error {
	if n < 0 || c.remaining() < n {
		return fmt.Errorf("unexpected end of buffer (need %v bytes at offset %v, have %v)", n, c.pos, c.remaining())
	}
	return nil
}

func (c *cursor) u32() (uint32, error) {
	if err := c.need(4); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint32(c.b[c.pos:])
	c.pos += 4
	return v, nil
}

func (c *cursor) i32() (int32, error) {
	v, err := c.u32()
	return int32(v), err
}
