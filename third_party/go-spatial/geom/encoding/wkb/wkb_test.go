package wkb_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb"
	"github.com/go-spatial/geom/encoding/wkb/internal/decode"
)

// leHeader builds a little-endian WKB geometry header (byte order marker +
// geometry type).
func leHeader(typ uint32) []byte {
	b := make([]byte, 5)
	b[0] = 1 // little-endian
	binary.LittleEndian.PutUint32(b[1:], typ)
	return b
}

// hostileCount builds a geometry whose header declares the maximum element
// count but whose body is only a few bytes: decoding must fail with a count
// error instead of allocating slices for the declared count.
func hostileCount(typ uint32) []byte {
	b := leHeader(typ)
	b = binary.LittleEndian.AppendUint32(b, 0xFFFFFFFF)
	// 10 bytes of body: nowhere near enough for any element.
	return append(b, make([]byte, 10)...)
}

// TestDecodeBytesHostileCounts verifies that a huge element/point count in a
// WKB header is rejected against the remaining input size before any
// allocation is made (defence against OOM on corrupt or hostile data).
func TestDecodeBytesHostileCounts(t *testing.T) {
	tests := []struct {
		name string
		typ  uint32
	}{
		{"linestring", wkb.LineString},
		{"polygon", wkb.Polygon},
		{"multipoint", wkb.MultiPoint},
		{"multilinestring", wkb.MultiLineString},
		{"multipolygon", wkb.MultiPolygon},
		{"collection", wkb.Collection},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := wkb.DecodeBytes(hostileCount(tc.typ))
			if err == nil {
				t.Fatal("expected an error for a hostile element count, got none")
			}
			var countErr decode.ErrElementCount
			if !errors.As(err, &countErr) {
				t.Fatalf("expected decode.ErrElementCount, got %T: %v", err, err)
			}
			if countErr.Count != 0xFFFFFFFF {
				t.Fatalf("expected the declared count 0xFFFFFFFF in the error, got %v", countErr.Count)
			}
		})
	}
}

// TestDecodeBytesCountExceedsRemaining covers counts that are large but not
// saturated and nested geometries whose inner count is the hostile one.
func TestDecodeBytesCountExceedsRemaining(t *testing.T) {
	// 3 points declared, only 2 present: 3*16 > 32 remaining bytes.
	b := leHeader(wkb.LineString)
	b = binary.LittleEndian.AppendUint32(b, 3)
	b = append(b, make([]byte, 32)...)
	if _, err := wkb.DecodeBytes(b); err == nil {
		t.Fatal("expected an error when the point count exceeds the input size")
	} else {
		var countErr decode.ErrElementCount
		if !errors.As(err, &countErr) {
			t.Fatalf("expected decode.ErrElementCount, got %T: %v", err, err)
		}
	}

	// A multilinestring with one line entry whose own point count is hostile:
	// the outer count (1 element) fits, the inner count must be caught.
	nested := leHeader(wkb.MultiLineString)
	nested = binary.LittleEndian.AppendUint32(nested, 1)
	nested = append(nested, leHeader(wkb.LineString)...)
	nested = binary.LittleEndian.AppendUint32(nested, 0xFFFFFFFF)
	nested = append(nested, make([]byte, 10)...)
	if _, err := wkb.DecodeBytes(nested); err == nil {
		t.Fatal("expected an error for a hostile nested point count")
	} else {
		var countErr decode.ErrElementCount
		if !errors.As(err, &countErr) {
			t.Fatalf("expected decode.ErrElementCount, got %T: %v", err, err)
		}
	}
}

// TestDecodeBytesValidRoundTrip ensures the count guards do not reject valid
// geometries, including empty ones.
func TestDecodeBytesValidRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		g    geom.Geometry
	}{
		{"point", geom.Point{1.5, -2.25}},
		{"empty linestring", geom.LineString{}},
		{"linestring", geom.LineString{{0, 0}, {1, 1}, {2, 4}}},
		{"polygon", geom.Polygon{{{0, 0}, {0, 1}, {1, 1}, {0, 0}}}},
		{"multipoint", geom.MultiPoint{{0, 0}, {1, 1}}},
		{"multilinestring", geom.MultiLineString{{{0, 0}, {1, 1}}}},
		{"multipolygon", geom.MultiPolygon{{{{0, 0}, {0, 1}, {1, 1}, {0, 0}}}}},
		{"collection", geom.Collection{geom.Point{0, 0}, geom.LineString{{0, 0}, {1, 1}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bs, err := wkb.EncodeBytes(tc.g)
			if err != nil {
				t.Fatalf("EncodeBytes: %v", err)
			}
			if _, err := wkb.DecodeBytes(bs); err != nil {
				t.Fatalf("DecodeBytes(EncodeBytes(%v)): %v", tc.name, err)
			}
		})
	}
}

// FuzzDecodeBytes ensures arbitrary input never panics or drives oversized
// allocations through the element/point counters.
func FuzzDecodeBytes(f *testing.F) {
	// Seeds: a valid point, a valid linestring and hostile count headers for
	// every geometry type carrying a count.
	point := leHeader(wkb.Point)
	point = binary.LittleEndian.AppendUint64(point, 0x3FF8000000000000) // 1.5
	point = binary.LittleEndian.AppendUint64(point, 0xC002000000000000) // -2.25
	f.Add(point)

	ln := leHeader(wkb.LineString)
	ln = binary.LittleEndian.AppendUint32(ln, 2)
	ln = append(ln, make([]byte, 32)...)
	f.Add(ln)

	for _, typ := range []uint32{
		wkb.LineString, wkb.Polygon, wkb.MultiPoint,
		wkb.MultiLineString, wkb.MultiPolygon, wkb.Collection,
	} {
		f.Add(hostileCount(typ))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = wkb.DecodeBytes(data)
	})
}

// nestedCollection builds a WKB GeometryCollection nested depth levels deep:
// every level is a collection with one element and the innermost element is a
// point.
func nestedCollection(depth int) []byte {
	b := leHeader(wkb.Point)
	b = binary.LittleEndian.AppendUint64(b, 0x3FF8000000000000) // 1.5
	b = binary.LittleEndian.AppendUint64(b, 0xC002000000000000) // -2.25
	for i := 0; i < depth; i++ {
		wrapped := leHeader(wkb.Collection)
		wrapped = binary.LittleEndian.AppendUint32(wrapped, 1)
		wrapped = append(wrapped, b...)
		b = wrapped
	}
	return b
}

// lenHidingReader wraps a reader so the decoder's remaining-size pre-check
// (which probes for a Len() method) cannot run, exercising the code paths that
// handle opaque io.Readers.
type lenHidingReader struct{ io.Reader }

// TestDecodeNestedCollectionDepthLimit covers audit P5-4: deeply nested
// GeometryCollections must be rejected with a clear error instead of recursing
// until the stack is exhausted. Nesting up to decode.MaxNestingDepth levels
// decodes fine; one level more fails with decode.ErrMaxNestingDepth. Both the
// sized (DecodeBytes) and plain io.Reader paths are checked.
func TestDecodeNestedCollectionDepthLimit(t *testing.T) {
	atLimit := nestedCollection(decode.MaxNestingDepth)
	if _, err := wkb.DecodeBytes(atLimit); err != nil {
		t.Fatalf("DecodeBytes with %d nested collections: %v, want success", decode.MaxNestingDepth, err)
	}
	if _, err := wkb.Decode(lenHidingReader{bytes.NewReader(atLimit)}); err != nil {
		t.Fatalf("Decode with %d nested collections: %v, want success", decode.MaxNestingDepth, err)
	}

	tooDeep := nestedCollection(decode.MaxNestingDepth + 1)
	tests := []struct {
		name string
		run  func() error
	}{
		{"sized reader", func() error {
			_, err := wkb.DecodeBytes(tooDeep)
			return err
		}},
		{"plain reader", func() error {
			_, err := wkb.Decode(lenHidingReader{bytes.NewReader(tooDeep)})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected an error for %d nested collections, got none", decode.MaxNestingDepth+1)
			}
			var depthErr decode.ErrMaxNestingDepth
			if !errors.As(err, &depthErr) {
				t.Fatalf("expected decode.ErrMaxNestingDepth, got %T: %v", err, err)
			}
			if depthErr.Max != decode.MaxNestingDepth {
				t.Fatalf("error reports max %d, want %d", depthErr.Max, decode.MaxNestingDepth)
			}
		})
	}
}

// TestDecodeUnsizedReaderElementCap covers audit P5-4: for readers that do
// not report their remaining length the element-count pre-check cannot run,
// so declared counts must be capped at decode.MaxElements to keep hostile
// input from forcing huge allocations.
func TestDecodeUnsizedReaderElementCap(t *testing.T) {
	b := leHeader(wkb.LineString)
	b = binary.LittleEndian.AppendUint32(b, decode.MaxElements+1)
	b = append(b, make([]byte, 64)...)

	_, err := wkb.Decode(lenHidingReader{bytes.NewReader(b)})
	if err == nil {
		t.Fatal("expected an error for a declared count above MaxElements, got none")
	}
	var countErr decode.ErrElementCount
	if !errors.As(err, &countErr) {
		t.Fatalf("expected decode.ErrElementCount, got %T: %v", err, err)
	}
	if countErr.Count != decode.MaxElements+1 {
		t.Fatalf("expected the declared count %d in the error, got %v", decode.MaxElements+1, countErr.Count)
	}
}
