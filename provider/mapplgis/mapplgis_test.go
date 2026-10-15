package mapplgis

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/mos"
)

// buildBlob encodes a minimal valid LayerSystemInfo blob with the given
// units and precision.
func buildBlob(precision int, units byte, unitsDefined bool, projection string) []byte {
	buf := make([]byte, 64+len(projection))
	copy(buf, []byte{5, 'V', 'e', 'r', ' ', '1'})
	le := func(off int, v uint32) {
		buf[off] = byte(v)
		buf[off+1] = byte(v >> 8)
		buf[off+2] = byte(v >> 16)
		buf[off+3] = byte(v >> 24)
	}
	le(11, uint32(precision))
	buf[15] = 1 // flProjection
	le(26, 42)  // LayerID
	le(60, uint32(len(projection)))
	copy(buf[64:], projection)
	buf[52] = units
	if unitsDefined {
		buf[53] = 1
	}
	return buf
}

func fullColumns() []string {
	return []string{"OKEY", "MUID", "MINX", "MAXX", "MINY", "MAXY", "ObjectStyle", "ObjectType", "LINE"}
}

func fullIndexes() []IndexMeta {
	idx := make([]IndexMeta, 0, len(requiredIndexes))
	for _, c := range requiredIndexes {
		idx = append(idx, IndexMeta{Name: "I_" + strings.ToUpper(c), Columns: []string{c}})
	}
	return idx
}

func validFetcher() SystemInfoFetcher {
	blob := buildBlob(0, 0, true, "")
	return func() (*mos.SystemInfo, error) {
		si, err := mos.ParseSystemInfo(blob)
		if err != nil {
			return nil, err
		}
		return &si, nil
	}
}

func TestDetect(t *testing.T) {
	t.Run("full contract passes", func(t *testing.T) {
		info, err := Detect(TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY"}, Indexes: fullIndexes()}, validFetcher())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.IsMapplGIS {
			t.Fatal("expected IsMapplGIS=true")
		}
		if info.SystemInfo.LayerID != 42 {
			t.Errorf("expected LayerID 42, got %d", info.SystemInfo.LayerID)
		}
	})

	t.Run("case-insensitive names", func(t *testing.T) {
		cols := []string{"okey", "muid", "minx", "maxx", "miny", "maxy", "objectstyle", "objecttype", "line"}
		meta := TableMeta{Columns: cols, PrimaryKeyColumns: []string{"Okey"}, Indexes: []IndexMeta{
			{Name: "i1", Columns: []string{"muid"}},
			{Name: "i2", Columns: []string{"MinX"}},
			{Name: "i3", Columns: []string{"maxx"}},
			{Name: "i4", Columns: []string{"MiNy"}},
			{Name: "i5", Columns: []string{"MAXY"}},
			{Name: "i6", Columns: []string{"objecttype"}},
		}}
		info, err := Detect(meta, validFetcher())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.IsMapplGIS {
			t.Fatal("expected IsMapplGIS=true with mixed-case metadata")
		}
	})

	t.Run("missing required column", func(t *testing.T) {
		for _, drop := range fullColumns() {
			var cols []string
			for _, c := range fullColumns() {
				if c != drop {
					cols = append(cols, c)
				}
			}
			meta := TableMeta{Columns: cols, PrimaryKeyColumns: []string{"OKEY"}, Indexes: fullIndexes()}
			info, err := Detect(meta, func() (*mos.SystemInfo, error) {
				t.Fatalf("OKEY=1 query must not run when a required column is missing")
				return nil, nil
			})
			if err != nil {
				t.Fatalf("unexpected error dropping %v: %v", drop, err)
			}
			if info.IsMapplGIS {
				t.Errorf("expected IsMapplGIS=false when %v missing", drop)
			}
		}
	})

	t.Run("OKEY not primary key", func(t *testing.T) {
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"MUID"}, Indexes: fullIndexes()}
		info, err := Detect(meta, validFetcher())
		if err != nil || info.IsMapplGIS {
			t.Errorf("expected not-detected with non-OKEY PK (info=%v err=%v)", info, err)
		}
	})

	t.Run("composite primary key fails", func(t *testing.T) {
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY", "MUID"}, Indexes: fullIndexes()}
		info, err := Detect(meta, validFetcher())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.IsMapplGIS {
			t.Error("expected IsMapplGIS=false with composite PK (OKEY, MUID)")
		}
	})

	t.Run("no primary key", func(t *testing.T) {
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: nil, Indexes: fullIndexes()}
		info, err := Detect(meta, validFetcher())
		if err != nil || info.IsMapplGIS {
			t.Errorf("expected not-detected without PK (info=%v err=%v)", info, err)
		}
	})

	t.Run("each required index dropped", func(t *testing.T) {
		for _, drop := range requiredIndexes {
			var idx []IndexMeta
			for _, c := range fullIndexes() {
				if !strings.EqualFold(c.Columns[0], drop) {
					idx = append(idx, c)
				}
			}
			meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY"}, Indexes: idx}
			info, err := Detect(meta, validFetcher())
			if err != nil {
				t.Fatalf("unexpected error dropping index %v: %v", drop, err)
			}
			if info.IsMapplGIS {
				t.Errorf("expected IsMapplGIS=false without index on %v", drop)
			}
		}
	})

	t.Run("composite index does not cover its columns", func(t *testing.T) {
		// All required fields indexed, but MINX/MAXX/MINY/MAXY share one
		// composite index instead of dedicated single-column indexes.
		idx := []IndexMeta{
			{Name: "i_muid", Columns: []string{"MUID"}},
			{Name: "i_bbox", Columns: []string{"MINX", "MAXX", "MINY", "MAXY"}},
			{Name: "i_otype", Columns: []string{"OBJECTTYPE"}},
		}
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY"}, Indexes: idx}
		info, err := Detect(meta, validFetcher())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.IsMapplGIS {
			t.Error("expected IsMapplGIS=false when bbox fields share one composite index")
		}
	})

	t.Run("field at second index position does not count", func(t *testing.T) {
		// MUID only appears at position two of a composite index.
		idx := []IndexMeta{
			{Name: "i_c", Columns: []string{"OBJECTSTYLE", "MUID"}},
			{Name: "i2", Columns: []string{"MINX"}},
			{Name: "i3", Columns: []string{"MAXX"}},
			{Name: "i4", Columns: []string{"MINY"}},
			{Name: "i5", Columns: []string{"MAXY"}},
			{Name: "i6", Columns: []string{"OBJECTTYPE"}},
		}
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY"}, Indexes: idx}
		info, err := Detect(meta, validFetcher())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.IsMapplGIS {
			t.Error("expected IsMapplGIS=false when MUID is only the second index column")
		}
	})

	t.Run("OKEY=1 row missing", func(t *testing.T) {
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY"}, Indexes: fullIndexes()}
		info, err := Detect(meta, func() (*mos.SystemInfo, error) {
			return nil, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.IsMapplGIS {
			t.Error("expected IsMapplGIS=false when the OKEY=1 row is absent")
		}
	})

	t.Run("invalid LINE blob is a controlled error", func(t *testing.T) {
		meta := TableMeta{Columns: fullColumns(), PrimaryKeyColumns: []string{"OKEY"}, Indexes: fullIndexes()}
		_, err := Detect(meta, func() (*mos.SystemInfo, error) {
			return nil, fmt.Errorf("not a system info blob")
		})
		if err == nil {
			t.Fatal("expected an error for an unparseable LINE blob, not a silent miss")
		}
	})
}

func TestRequiredColumnListMatchesGeometryField(t *testing.T) {
	found := false
	for _, c := range requiredColumns {
		if strings.EqualFold(c, GeometryField) {
			found = true
		}
	}
	if !found {
		t.Errorf("GeometryField %v must be among the required columns", GeometryField)
	}
}

func TestPrimaryKeyConstant(t *testing.T) {
	for _, c := range requiredColumns {
		if strings.EqualFold(c, PrimaryKey) {
			return
		}
	}
	t.Errorf("PrimaryKey %v must be among the required columns", PrimaryKey)
}
