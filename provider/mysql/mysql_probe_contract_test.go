package mysql

import (
	"fmt"
	"strings"
	"testing"

	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
)

// TestProbeMOSCustomSQLContract runs the shared bounds-contract fixture
// through the real MySQL probe (real rows.Scan — audit A01) and asserts the
// identical SQLGeometryContract every other provider reports for the same
// fixture.
func TestProbeMOSCustomSQLContract(t *testing.T) {
	probe := func(t *testing.T, columns []string, rows [][]interface{}, format string) ([]string, codec.SQLGeometryContract, *Layer) {
		t.Helper()
		var queries []string
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: fixture.DriverRows(rows), QueryLog: &queries})
		layer := &Layer{
			name:          "probe_layer",
			geomFieldname: "geom",
			bboxFields:    codec.DefaultBBoxFields(),
			mosConfig:     mosfixture.Config(),
		}
		cols, contract, err := probeMOSCustomSQLContract(db, layer, "SELECT * FROM probe_table", format, "mysql")
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		wantQueries := 2
		if format == codec.FormatMOS {
			wantQueries = 1
		}
		if len(queries) != wantQueries {
			t.Fatalf("expected %d probes, got %v", wantQueries, queries)
		}
		if format == codec.FormatMOS {
			if strings.Contains(queries[0], "__tegola_") || !strings.HasSuffix(strings.TrimSpace(queries[0]), "LIMIT 0") {
				t.Fatalf("metadata query must limit the original SELECT without materializing a sample: %v", queries)
			}
		} else if !strings.HasSuffix(strings.TrimSpace(queries[1]), "LIMIT 16") ||
			!strings.Contains(queries[1], "LEFT(CAST(") {
			t.Fatalf("format inference must retain its bounded sample: %v", queries)
		}
		return cols, contract, layer
	}

	t.Run("explicit MOS inspects metadata without decoding valid rows", func(t *testing.T) {
		_, c, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), codec.FormatMOS)
		if !c.HasBounds || c.GeometryField != "geom" || c.ValidRows != 0 || c.ValidMOSRows != 0 {
			t.Fatalf("unexpected explicit MOS contract: %+v", c)
		}
	})

	for _, badFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("short auto MOS badFirst=%v", badFirst), func(t *testing.T) {
			rows := mosfixture.ValidRows()[:1]
			_, c, _ := probe(t, mosfixture.Columns(), rows, "")
			if !c.DetectsMOS() || !c.SampleComplete || c.ValidMOSRows != 1 {
				t.Fatalf("single MOS: %+v", c)
			}
			bad := mosfixture.RowsWithMalformed()[0]
			if badFirst {
				rows = append([][]interface{}{bad}, rows...)
			} else {
				rows = append(rows, bad)
			}
			_, c, _ = probe(t, mosfixture.Columns(), rows, "")
			if !c.DetectsMOS() || c.ValidMOSRows != 1 {
				t.Fatalf("mixed short MOS: %+v", c)
			}
		})
	}
	t.Run("three real MOS rows report the identical shared contract", func(t *testing.T) {
		cols, contract, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), "")
		for i, c := range mosfixture.Columns() {
			if cols[i] != c {
				t.Fatalf("column %d = %q, expected %q", i, cols[i], c)
			}
		}
		if contract != mosfixture.ExpectedContract() {
			t.Fatalf("contract %+v, expected identical fixture contract %+v", contract, mosfixture.ExpectedContract())
		}
	})

	t.Run("one malformed row plus three valid rows counts three", func(t *testing.T) {
		_, contract, _ := probe(t, mosfixture.Columns(), mosfixture.RowsWithMalformed(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (malformed rows skipped, never counted)", contract.ValidMOSRows)
		}
		if contract.ValidRows != 3 {
			t.Fatalf("ValidRows = %d, expected 3", contract.ValidRows)
		}
	})

	t.Run("system info row is skipped and never applied", func(t *testing.T) {
		_, contract, layer := probe(t, mosfixture.Columns(), mosfixture.RowsWithSystemInfo(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (system info row skipped)", contract.ValidMOSRows)
		}
		// zero config mutation: the probe never applies the system-info row
		if layer.mosConfig != mosfixture.Config() {
			t.Fatalf("mosConfig mutated by probe: %+v", layer.mosConfig)
		}
	})

	t.Run("lower-case bounds columns persist actual names", func(t *testing.T) {
		_, contract, _ := probe(t, mosfixture.LowerCaseColumns(), mosfixture.ValidRows(), "")
		if contract != mosfixture.ExpectedLowerCaseContract() {
			t.Fatalf("contract %+v, expected %+v (actual result-column names)", contract, mosfixture.ExpectedLowerCaseContract())
		}
	})

	// audit part11 7.1.3 regression: native rows never count as MOS rows,
	// so native-geometry tables with bounds columns are never detected as
	// MapplGIS sql-sample and never switch to mos.
	t.Run("native geometry rows never count as MOS rows", func(t *testing.T) {
		_, contract, _ := probe(t, mosfixture.Columns(), mosfixture.NativeRows(), "")
		if contract.ValidMOSRows != 0 {
			t.Fatalf("ValidMOSRows = %d, expected 0 (native rows must not count as MOS)", contract.ValidMOSRows)
		}
		if contract.ValidMOSRows >= codec.MinValidMOSRows {
			t.Fatal("native fixture must not produce MOS evidence")
		}
	})

	// audit A04: the inference (auto) probe decode closure must fall back
	// to codec.DecodeMOS so MOS rows are recognized even when the native
	// decoder rejects them.
	t.Run("auto format probe falls back to MOS decode", func(t *testing.T) {
		_, contract, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (auto decode must fall back to DecodeMOS)", contract.ValidMOSRows)
		}
	})
}
