// Package mapplgis implements the canonical one-time MapplGIS table
// detection contract shared by all standard providers.
//
// A table is recognized as a MapplGIS layer only when ALL of the following
// hold (checked case-insensitively):
//
//  1. Its DDL contains the columns OKEY, MUID, MINX, MAXX, MINY, MAXY,
//     ObjectStyle, ObjectType and LINE.
//  2. OKEY is the primary key — the primary key consists of exactly one
//     column, and that column is OKEY.
//  3. Each of MUID, MINX, MAXX, MINY, MAXY and ObjectType is covered by a
//     dedicated single-column index (an index whose column list is exactly
//     that one column). Composite indexes do not satisfy the contract, and
//     a column at position two or later of any index does not count.
//  4. The row OKEY = 1 carries a non-empty LINE value that parses as a
//     MapplGIS LayerSystemInfo blob.
//
// Detection runs once per tablename layer at provider registration; tile
// requests never repeat it. Custom SQL layers skip the canonical checks
// above entirely: they are recognized only through the provider's SQL-sample
// contract (MapplGISSQLSample), which requires bounds columns, a geometry
// column and 3 valid MOS sample rows, or at least one in a completed short
// result. Explicit MOS only checks metadata and does not get this sample tag. A custom SQL layer is tagged
// on a successful MOS sample only, and SystemInfo rows are never applied for
// it.
package mapplgis

import (
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/mos"
)

// GeometryField is the geometry column of a detected MapplGIS table.
const GeometryField = "LINE"

// PrimaryKey is the single primary key column of a detected MapplGIS table.
const PrimaryKey = "OKEY"

// requiredColumns lists the DDL columns a MapplGIS table must carry.
var requiredColumns = []string{
	"OKEY", "MUID", "MINX", "MAXX", "MINY", "MAXY",
	"OBJECTSTYLE", "OBJECTTYPE", "LINE",
}

// requiredIndexes lists the columns that must each be covered by a
// dedicated single-column index.
var requiredIndexes = []string{
	"MUID", "MINX", "MAXX", "MINY", "MAXY", "OBJECTTYPE",
}

// IndexMeta describes one table index.
type IndexMeta struct {
	// Name is the index name in its stored case.
	Name string
	// Columns holds the indexed columns in index-position order.
	Columns []string
}

// TableMeta is the backend-collected schema metadata fed to Detect. Each
// provider populates it with its own introspection mechanism.
type TableMeta struct {
	// Columns holds every column name of the table in its stored case.
	Columns []string
	// PrimaryKeyColumns holds the primary key columns in key order. A
	// table without a primary key leaves it empty.
	PrimaryKeyColumns []string
	// Indexes describes every index of the table.
	Indexes []IndexMeta
}

// SystemInfoFetcher reads the OKEY = 1 metadata row and parses its LINE
// value as a LayerSystemInfo blob. It returns (nil, nil) when no such row
// exists, the parsed blob when it does, and an error when the row exists
// but cannot be parsed.
type SystemInfoFetcher func() (*mos.SystemInfo, error)

// Info is the immutable detection result stored on the layer.
type Info struct {
	// IsMapplGIS reports whether the table satisfied the full contract.
	IsMapplGIS bool
	// SystemInfo is the parsed layer self-description. Valid only when
	// IsMapplGIS is true.
	SystemInfo mos.SystemInfo
}

// Detect applies the canonical MapplGIS table contract to the given schema
// metadata. It returns (Info{}, nil) when the table is not a MapplGIS layer
// and a controlled error when the metadata row exists but is invalid —
// never a silent true on partial evidence.
func Detect(meta TableMeta, fetch SystemInfoFetcher) (Info, error) {
	columns := make(map[string]struct{}, len(meta.Columns))
	for _, c := range meta.Columns {
		columns[strings.ToLower(c)] = struct{}{}
	}
	for _, req := range requiredColumns {
		if _, ok := columns[strings.ToLower(req)]; !ok {
			return Info{}, nil
		}
	}

	// The primary key must be exactly [OKEY]: a composite (OKEY, MUID)
	// key or a single non-OKEY key both fail.
	if len(meta.PrimaryKeyColumns) != 1 || !strings.EqualFold(meta.PrimaryKeyColumns[0], PrimaryKey) {
		return Info{}, nil
	}

	// Each required field needs a dedicated single-column index. A
	// composite index covering the field does not count, and a field
	// at index position two or later of any index does not count.
	single := make(map[string]struct{}, len(meta.Indexes))
	for _, idx := range meta.Indexes {
		if len(idx.Columns) == 1 {
			single[strings.ToLower(idx.Columns[0])] = struct{}{}
		}
	}
	for _, req := range requiredIndexes {
		if _, ok := single[strings.ToLower(req)]; !ok {
			return Info{}, nil
		}
	}

	sysInfo, err := fetch()
	if err != nil {
		return Info{}, fmt.Errorf("mapplgis: reading OKEY=1 layer system info: %w", err)
	}
	if sysInfo == nil {
		// No OKEY = 1 metadata row: the table carries the MapplGIS DDL
		// but not the layer self-description, so the contract fails.
		return Info{}, nil
	}

	return Info{IsMapplGIS: true, SystemInfo: *sysInfo}, nil
}
