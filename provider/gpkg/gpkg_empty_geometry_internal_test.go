//go:build cgo

package gpkg

// Unit tests for audit part12 N15: decodeGeometry must honor the
// GeoPackage binary header empty-geometry flag instead of passing the
// (usually empty) remainder to the WKB decoder.

import (
	"testing"

	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// gpkgHeader builds a minimal GeoPackage binary header: magic GP, given
// flags byte, and the SRS id encoded in the header byte order. No
// envelope (envelope indicator 0).
func gpkgHeader(flags byte, srsid uint32) []byte {
	h := []byte{'G', 'P', 0x00, flags}
	if flags&0x01 == 0 { // big endian
		h = append(h, byte(srsid>>24), byte(srsid>>16), byte(srsid>>8), byte(srsid))
	} else { // little endian
		h = append(h, byte(srsid), byte(srsid>>8), byte(srsid>>16), byte(srsid>>24))
	}
	return h
}

// wkbPointLE is a little-endian WKB point (0 0).
var wkbPointLE = []byte{
	0x01,
	0x01, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func TestDecodeGeometryEmptyFlagSkipsWKB(t *testing.T) {
	const emptyFlag = 1 << 4 // maskEmptyGeometry, big-endian header

	t.Run("empty flag with garbage tail decodes to nil geometry", func(t *testing.T) {
		blob := append(gpkgHeader(emptyFlag, 3857), 0x01, 0x02, 0x03)
		h, geo, err := decodeGeometry(blob)
		if err != nil {
			t.Fatalf("decodeGeometry: %v", err)
		}
		if geo != nil {
			t.Errorf("expected nil geometry for an empty-flagged feature, got %v", geo)
		}
		if h == nil || !h.IsGeometryEmpty() {
			t.Errorf("expected the header to report the empty geometry flag: %v", h)
		}
		if h.SRSId() != 3857 {
			t.Errorf("header SRS id = %v, expected 3857", h.SRSId())
		}
	})

	t.Run("empty flag wins over a valid WKB tail", func(t *testing.T) {
		blob := append(gpkgHeader(emptyFlag|0x01, 3857), wkbPointLE...)
		h, geo, err := decodeGeometry(blob)
		if err != nil {
			t.Fatalf("decodeGeometry: %v", err)
		}
		if geo != nil {
			t.Errorf("expected the empty flag to short-circuit WKB decoding, got %v", geo)
		}
		if h == nil || !h.IsGeometryEmpty() {
			t.Errorf("expected the header to report the empty geometry flag: %v", h)
		}
	})

	t.Run("without the flag the same tail is still a decode error", func(t *testing.T) {
		blob := append(gpkgHeader(0x00, 3857), 0x01, 0x02, 0x03)
		if _, _, err := decodeGeometry(blob); err == nil {
			t.Fatal("expected a WKB decode error without the empty flag, got nil")
		}
	})

	t.Run("non-empty features decode normally", func(t *testing.T) {
		blob := append(gpkgHeader(0x01, 3857), wkbPointLE...)
		h, geo, err := decodeGeometry(blob)
		if err != nil {
			t.Fatalf("decodeGeometry: %v", err)
		}
		if geo == nil {
			t.Fatal("expected a decoded geometry")
		}
		if h == nil || h.IsGeometryEmpty() {
			t.Errorf("header must not report the empty flag: %v", h)
		}
	})

	t.Run("decodeGeometryValue default format passes the flag through", func(t *testing.T) {
		blob := append(gpkgHeader(emptyFlag, 3857), 0x01, 0x02, 0x03)
		for _, format := range []string{"", GeometryFormatGPKG} {
			_, geo, err := decodeGeometryValue(blob, format, codec.MOSConfig{})
			if err != nil {
				t.Fatalf("decodeGeometryValue(%q): %v", format, err)
			}
			if geo != nil {
				t.Errorf("decodeGeometryValue(%q): expected nil geometry, got %v", format, geo)
			}
		}
	})

	t.Run("wkb tail starts right after the 8 byte header", func(t *testing.T) {
		// regression guard for BinaryHeader.Size(): an 8-byte header with
		// no envelope must not leave header bytes in the WKB stream.
		blob := append(gpkgHeader(0x01, 3857), wkbPointLE...)
		if got := len(blob); got != 8+len(wkbPointLE) {
			t.Fatalf("fixture size = %d, expected %d", got, 8+len(wkbPointLE))
		}
		h, _, err := decodeGeometry(blob)
		if err != nil {
			t.Fatalf("decodeGeometry: %v", err)
		}
		if h.Size() != 8 {
			t.Errorf("header size = %d, expected 8", h.Size())
		}
	})
}

// TestDecodeGeometryEmptyFlagIsIdempotent guards the header flag reader
// itself (IsGeometryEmpty) against bit regressions. Only envelope
// indicator 0 (none) is used so the fixtures stay 8-byte headers.
func TestDecodeGeometryEmptyFlagIsIdempotent(t *testing.T) {
	for _, flags := range []byte{1 << 4, 1<<4 | 0x01} {
		h, err := NewBinaryHeader(gpkgHeader(flags, 4326))
		if err != nil {
			t.Fatalf("NewBinaryHeader(0x%02x): %v", flags, err)
		}
		if !h.IsGeometryEmpty() {
			t.Errorf("flags 0x%02x: expected IsGeometryEmpty", flags)
		}
	}
	for _, flags := range []byte{0x00, 0x01} {
		h, err := NewBinaryHeader(gpkgHeader(flags, 4326))
		if err != nil {
			t.Fatalf("NewBinaryHeader(0x%02x): %v", flags, err)
		}
		if h.IsGeometryEmpty() {
			t.Errorf("flags 0x%02x: unexpected IsGeometryEmpty", flags)
		}
	}
}
