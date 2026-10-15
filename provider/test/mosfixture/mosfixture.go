// Package mosfixture provides the shared bounds-contract probe fixture used
// by the gpkg, mysql, postgis and hana probe-contract test suites. One
// fixture, one expected SQLGeometryContract: every provider's probe must
// report the identical contract for identical rows (audit part10 item A01).
package mosfixture

import (
	"encoding/binary"
	"math"

	"github.com/alexeydott/tegola/mos"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// Columns returns the result-column names of the reference fixture. The
// bounds columns use the canonical uppercase default names.
func Columns() []string {
	return []string{"OKEY", "MINX", "MAXX", "MINY", "MAXY", "geom"}
}

// LowerCaseColumns returns the same fixture with lower-case bounds column
// names, the PostGIS-style case-insensitivity fixture (audit A09).
func LowerCaseColumns() []string {
	return []string{"okey", "minx", "maxx", "miny", "maxy", "geom"}
}

// Config returns the MOS config the fixture blobs are built for: precision 0
// so raw coordinates pass through unquantized.
func Config() codec.MOSConfig {
	return codec.MOSConfig{Precision: 0, UnitFactor: 1}
}

// MOSBlob assembles a valid MOS point blob (one sub-object, two points at
// (0,0) and (x,y)) using the native 10-byte header layout. The blob carries
// a positive MOS signature (mos.DecodeHeader succeeds).
func MOSBlob(x, y int32) []byte {
	buf := make([]byte, 0, 26)
	buf = append(buf, mos.TypePoint, 0)            // type, mod
	buf = binary.LittleEndian.AppendUint16(buf, 0) // add flag
	buf = binary.LittleEndian.AppendUint16(buf, 1) // sub-object count
	buf = binary.LittleEndian.AppendUint32(buf, 2) // total points
	buf = binary.LittleEndian.AppendUint32(buf, 2) // per-sub-object point count
	buf = binary.LittleEndian.AppendUint32(buf, 0) // (0,0)
	buf = binary.LittleEndian.AppendUint32(buf, 0)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(x)) // (x,y)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(y))
	return buf
}

// MalformedBlob returns a value that carries no valid MOS signature and
// decodes as neither native geometry nor MOS.
func MalformedBlob() []byte {
	return []byte{0xFF, 0x00}
}

// SystemInfoBlob assembles a MapplGIS LayerInfo (system info) blob: these
// rows must always be skipped by the probe and never applied.
func SystemInfoBlob() []byte {
	buf := make([]byte, 128)
	buf[0] = 5
	copy(buf[1:], "Ver 1")
	// precision 2
	buf[11] = 2
	buf[15] = 1 // flProjection
	buf[26] = 1 // LayerID
	buf[52] = byte(mos.UnitsMetres)
	buf[53] = 1 // flMapUnitsDefined
	return buf
}

// WKBPoint assembles a little-endian WKB point: the native-format regression
// fixture (audit part11 7.1.3) — a decodable native geometry that must NEVER
// count as a MOS row.
func WKBPoint(x, y float64) []byte {
	buf := make([]byte, 0, 21)
	buf = append(buf, 1) // little endian
	buf = binary.LittleEndian.AppendUint32(buf, 1)
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(x))
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(y))
	return buf
}

// ValidRows returns 3 rows carrying valid MOS geometries plus bounds.
func ValidRows() [][]interface{} {
	return [][]interface{}{
		{1, 0, 10, 0, 10, MOSBlob(5, 5)},
		{2, 5, 15, 5, 15, MOSBlob(10, 10)},
		{3, 9, 19, 9, 19, MOSBlob(15, 15)},
	}
}

// RowsWithMalformed returns 1 malformed row followed by 3 valid rows: the
// malformed row must be skipped, never counted.
func RowsWithMalformed() [][]interface{} {
	rows := [][]interface{}{
		{1, 0, 10, 0, 10, MalformedBlob()},
	}
	return append(rows, ValidRows()...)
}

// RowsWithSystemInfo returns 1 system-info row followed by 3 valid rows: the
// system-info row must be skipped, never counted or applied.
func RowsWithSystemInfo() [][]interface{} {
	rows := [][]interface{}{
		{0, 0, 10, 0, 10, SystemInfoBlob()},
	}
	return append(rows, ValidRows()...)
}

// NativeRows returns 3 rows carrying native WKB geometries plus bounds: the
// audit part11 7.1.3 regression fixture.
func NativeRows() [][]interface{} {
	return [][]interface{}{
		{1, 0, 10, 0, 10, WKBPoint(1, 1)},
		{2, 5, 15, 5, 15, WKBPoint(2, 2)},
		{3, 9, 19, 9, 19, WKBPoint(3, 3)},
	}
}

// ExpectedContract is the SQLGeometryContract every provider's probe must
// report for ValidRows against Columns() with the default bounds fields and
// geometry field "geom".
func ExpectedContract() codec.SQLGeometryContract {
	return codec.SQLGeometryContract{
		BoundsFields:  codec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
		GeometryField: "geom",
		ValidRows:     3,
		ValidMOSRows:  3,
		HasBounds:     true,
	}
}

// ExpectedLowerCaseContract is the expected contract for ValidRows against
// LowerCaseColumns: the probe must persist the ACTUAL result-column names.
func ExpectedLowerCaseContract() codec.SQLGeometryContract {
	return codec.SQLGeometryContract{
		BoundsFields:  codec.BBoxFields{"minx", "maxx", "miny", "maxy"},
		GeometryField: "geom",
		ValidRows:     3,
		ValidMOSRows:  3,
		HasBounds:     true,
	}
}
