package hana

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
)

// openContractStubLogged opens a fixture whose executed query
// texts are appended to the returned log (audit R1).
func openContractStubLogged(t *testing.T, columns []string, rows [][]driver.Value) (*sql.DB, *[]string) {
	t.Helper()
	queryLog := &[]string{}
	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows, QueryLog: queryLog})
	return db, queryLog
}

func openContractStub(t *testing.T, columns []string, rows [][]driver.Value) *sql.DB {
	t.Helper()
	db, _ := openContractStubLogged(t, columns, rows)
	return db
}

// openContractStubCounting is openContractStub plus a driver-rows close
// counter for asserting rows release (audit N10). Every column reports
// the BLOB database type (an accepted geometry/attribute type) so field
// introspection succeeds; the probe under test performs no row scanning.
func openContractStubCounting(t *testing.T, columns []string, rows [][]driver.Value) (*sql.DB, *int32) {
	t.Helper()
	closes := new(int32)
	typeNames := make([]string, len(columns))
	for i := range typeNames {
		typeNames[i] = "BLOB"
	}
	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows, Closes: closes, TypeNames: typeNames})
	return db, closes
}

// openContractStubCtxs is openContractStub plus a recorder for the context
// every query executes under (audit P5-16). Every column reports the BLOB
// database type so field introspection succeeds.
func openContractStubCtxs(t *testing.T, columns []string, rows [][]driver.Value) (*sql.DB, *[]context.Context) {
	t.Helper()
	ctxLog := &[]context.Context{}
	typeNames := make([]string, len(columns))
	for i := range typeNames {
		typeNames[i] = "BLOB"
	}
	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: columns, Rows: rows, ContextLog: ctxLog, TypeNames: typeNames})
	return db, ctxLog
}

// TestProbeMOSCustomSQLContract runs the shared bounds-contract fixture
// through the real HANA probe (standard row scan via setupRowValues + raw
// value unwrap — audit A01) and asserts the identical SQLGeometryContract
// the other providers report for the same fixture.
func TestProbeMOSCustomSQLContract(t *testing.T) {
	probe := func(t *testing.T, columns []string, rows [][]interface{}, format string) ([]string, codec.SQLGeometryContract, *Layer) {
		t.Helper()
		db, queries := openContractStubLogged(t, columns, fixture.DriverRows(rows))
		p := Provider{pool: &connectionPoolCollector{pool: db}}
		layer := &Layer{
			name:           "probe_layer",
			geomField:      "geom",
			bboxFields:     codec.DefaultBBoxFields(),
			mosConfig:      mosfixture.Config(),
			geometryFormat: format,
		}
		cols, contract, err := p.probeMOSCustomSQLContract(layer, "SELECT * FROM probe_table")
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		if format == codec.FormatMOS && (len(*queries) != 1 || !strings.HasSuffix((*queries)[0], "WHERE 1=0")) {
			t.Fatalf("not a metadata-only query: %v", *queries)
		}
		if format == codec.FormatMOS && strings.Contains((*queries)[0], "__tegola_bounds_probe") {
			t.Fatalf("metadata check must not wrap a sample query: %v", *queries)
		}
		if format != codec.FormatMOS && !strings.Contains((*queries)[0], "SELECT TOP 16") {
			t.Fatalf("format inference must retain its bounded sample: %v", *queries)
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
	})

	t.Run("system info row is skipped and never applied", func(t *testing.T) {
		_, contract, layer := probe(t, mosfixture.Columns(), mosfixture.RowsWithSystemInfo(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (system info row skipped)", contract.ValidMOSRows)
		}
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

	// audit part11 7.1.3 regression: native rows never count as MOS rows.
	t.Run("native geometry rows never count as MOS rows", func(t *testing.T) {
		_, contract, _ := probe(t, mosfixture.Columns(), mosfixture.NativeRows(), "")
		if contract.ValidMOSRows != 0 {
			t.Fatalf("ValidMOSRows = %d, expected 0 (native rows must not count as MOS)", contract.ValidMOSRows)
		}
	})

	// audit A04: the inference (auto) probe decode closure must fall back
	// to codec.DecodeMOS so MOS rows are recognized.
	t.Run("auto format probe falls back to MOS decode", func(t *testing.T) {
		_, contract, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (auto decode must fall back to DecodeMOS)", contract.ValidMOSRows)
		}
	})
}

// adversarialProbeSQL pins audit R1: TWO zoom tokens plus a bbox token
// inside a SQL function call — the historical preparation replaced only the
// first !ZOOM! occurrence and neutralized !BBOX! inconsistently.
const adversarialProbeSQL = `SELECT * FROM t WHERE min_zoom <= !ZOOM! AND max_zoom >= !ZOOM! AND geom && !BBOX! AND ST_Intersects(geom, !BBOX!) AND cx = !X!`

// TestInspectionProbeSQLR1 runs both HANA inspection probes end-to-end
// (audit R1) over adversarial custom SQL and asserts the executed query
// follows the shared codec.PrepareProbeSQL contract: every zoom/position
// token occurrence is neutralized, !BBOX! becomes "1=1" (never "TRUE", so
// it stays syntactically valid inside SQL function arguments) and the
// query uses the shared InspectionSampleLimit sample window
// (docs/provider-contract.md).
func TestInspectionProbeSQLR1(t *testing.T) {
	assertProbeSQL := func(t *testing.T, query string) {
		t.Helper()
		upper := strings.ToUpper(query)
		for _, tok := range []string{"!ZOOM!", "!BBOX!", "!X!", "!Y!"} {
			if strings.Contains(upper, tok) {
				t.Errorf("token %s left in probe SQL: %s", tok, query)
			}
		}
		if !strings.Contains(query, "1=1") {
			t.Errorf("bbox token must neutralize to 1=1: %s", query)
		}
		if strings.Contains(query, "TRUE") {
			t.Errorf("bbox token must never neutralize to TRUE (breaks function arguments): %s", query)
		}
		if !strings.Contains(query, "ST_Intersects(geom, 1=1)") {
			t.Errorf("ST_Intersects(geom, !BBOX!) must probe as ST_Intersects(geom, 1=1): %s", query)
		}
		if !strings.Contains(query, fmt.Sprintf("TOP %v", codec.InspectionSampleLimit)) {
			t.Errorf("probe SQL must use the shared sample window TOP %v: %s", codec.InspectionSampleLimit, query)
		}
	}

	newLayer := func() *Layer {
		return &Layer{
			name:           "probe_layer",
			sql:            adversarialProbeSQL,
			idField:        "gid",
			geomField:      "geom",
			bboxFields:     codec.DefaultBBoxFields(),
			mosConfig:      mosfixture.Config(),
			geometryFormat: codec.FormatMOS,
		}
	}

	t.Run("MOS probe flow succeeds over fixture rows", func(t *testing.T) {
		db, queryLog := openContractStubLogged(t, mosfixture.Columns(), fixture.DriverRows(mosfixture.ValidRows()))
		p := Provider{pool: &connectionPoolCollector{pool: db}}
		l := newLayer()
		if err := p.inspectMOSLayerGeomType(l); err != nil {
			t.Fatalf("probe over fixture rows: %v", err)
		}
		// DecodeMOS promotes the fixture WKB point to its multi-geometry
		// representation (MOS geometries decode as multi types).
		if _, ok := l.geomType.(geom.MultiPoint); !ok {
			t.Fatalf("geomType = %T, expected geom.MultiPoint", l.geomType)
		}
		if len(*queryLog) != 1 {
			t.Fatalf("executed %d queries, expected exactly 1: %v", len(*queryLog), *queryLog)
		}
		assertProbeSQL(t, (*queryLog)[0])
	})

	t.Run("native probe flow sniffs geometry type from sample rows", func(t *testing.T) {
		db, queryLog := openContractStubLogged(t, []string{"geom"}, [][]driver.Value{{"ST_Point"}})
		p := Provider{pool: &connectionPoolCollector{pool: db}}
		l := newLayer()
		l.geometryFormat = ""
		l.sql = "SELECT ST_AsBinary(geom) AS geom FROM t WHERE min_zoom <= !ZOOM! AND max_zoom >= !ZOOM! AND geom && !BBOX! AND ST_Intersects(geom, !BBOX!)"
		if err := p.inspectLayerGeomType("test_provider", l, nil); err != nil {
			t.Fatalf("probe over sample rows: %v", err)
		}
		if _, ok := l.geomType.(geom.Point); !ok {
			t.Fatalf("geomType = %T, expected geom.Point", l.geomType)
		}
		if len(*queryLog) != 1 {
			t.Fatalf("executed %d queries, expected exactly 1: %v", len(*queryLog), *queryLog)
		}
		assertProbeSQL(t, (*queryLog)[0])
	})
}

// TestMVTForLayersMissingLayerVsMissingBoundsContract pins the deliberate
// part12 0.6 behaviour change and keeps it separate from the bounds-contract
// gate: a MISSING LAYER now fails the MVT query instead of logging a warning
// and continuing with a zero-valued Layer (removed upstream behaviour), while
// custom SQL whose SELECT list omits the bounds columns must STILL register
// with a WARN only (b5ad0979) - the bounds gate never becomes fatal because
// of this fix.
func TestMVTForLayersMissingLayerVsMissingBoundsContract(t *testing.T) {
	t.Run("missing layer fails the query (part12 0.6)", func(t *testing.T) {
		// The nil pool proves the failure happens before any SQL runs: a
		// zero-valued Layer must never reach query construction.
		p := Provider{layers: map[string]Layer{}}
		_, err := p.MVTForLayers(context.Background(), nil, nil, []provider.Layer{{Name: "ghost"}})
		if err == nil {
			t.Fatal("MVTForLayers with an unregistered layer must fail (part12 0.6); warn-and-continue with a zero Layer is the removed upstream behaviour")
		}
		var lnf ErrLayerNotFound
		if !errors.As(err, &lnf) {
			t.Fatalf("missing layer must surface ErrLayerNotFound, got %T: %v", err, err)
		}
		if lnf.LayerName != "ghost" {
			t.Fatalf("ErrLayerNotFound.LayerName = %q, want %q", lnf.LayerName, "ghost")
		}
		// The message must name the layer and keep the missing-layer vs
		// missing-bounds distinction explicit.
		for _, want := range []string{"ghost", "missing layer fails the query"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error must contain %q, got: %v", want, err)
			}
		}
	})

	t.Run("missing bounds columns in custom SQL stay warn-only at registration (b5ad0979)", func(t *testing.T) {
		// Contrast half of the contract: custom SQL without bounds columns
		// in its result set registers with a WARN. The 0.6 fix is ONLY
		// about a missing layer; this gate must not start failing.
		contract := codec.SQLGeometryContract{GeometryField: "geom"}
		configured := codec.DefaultBBoxFields()
		resolved, boundsInResult, err := codec.ResolveBoundsSQLContract("l", "SELECT id, geom FROM t WHERE !BBOX!", "geom", contract, configured)
		if err != nil {
			t.Fatalf("missing bounds columns in the custom SQL result must stay warn-only at registration (b5ad0979), got error: %v", err)
		}
		if boundsInResult {
			t.Fatal("bounds absent from the result columns must report boundsInResult=false (warn + register)")
		}
		if resolved != configured {
			t.Fatalf("fallback must keep the configured bounds chain, got %v want %v", resolved, configured)
		}
	})
}
