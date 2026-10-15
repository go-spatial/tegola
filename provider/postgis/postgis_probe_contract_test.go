package postgis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// probeFakeRows is an in-memory pgx.Rows over REAL row values for the
// bounds-contract probe (audit A01).
type probeFakeRows struct {
	columns []string
	rows    [][]any
	next    int
}

func (r *probeFakeRows) Close() {}
func (r *probeFakeRows) Err() error {
	return nil
}
func (r *probeFakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}
func (r *probeFakeRows) FieldDescriptions() []pgconn.FieldDescription {
	fds := make([]pgconn.FieldDescription, len(r.columns))
	for i, c := range r.columns {
		fds[i] = pgconn.FieldDescription{Name: c}
	}
	return fds
}
func (r *probeFakeRows) Next() bool {
	if r.next >= len(r.rows) {
		return false
	}
	r.next++
	return true
}
func (r *probeFakeRows) Scan(dest ...any) error {
	return errors.New("probe consumes rows.Values, not Scan")
}
func (r *probeFakeRows) Values() ([]any, error) {
	return r.rows[r.next-1], nil
}
func (r *probeFakeRows) RawValues() [][]byte { return nil }
func (r *probeFakeRows) Conn() *pgx.Conn     { return nil }

// TypeMap satisfies pgx.Rows as of pgx v5.11.
func (r *probeFakeRows) TypeMap() *pgtype.Map { return nil }

// TestProbeSQLContractRows runs the shared bounds-contract fixture through
// the PostGIS probe row inspection and asserts the identical
// SQLGeometryContract the other providers report for the same fixture.
func TestProbeSQLContractRows(t *testing.T) {
	probe := func(t *testing.T, columns []string, rows [][]any, format string) (codec.SQLGeometryContract, *Layer) {
		t.Helper()
		layer := &Layer{
			name:           "probe_layer",
			geomField:      "geom",
			bboxFields:     codec.DefaultBBoxFields(),
			mosConfig:      mosfixture.Config(),
			geometryFormat: format,
		}
		fake := &probeFakeRows{columns: columns, rows: rows}
		_, contract, err := probeSQLContractRows(layer, fake)
		if format == codec.FormatMOS && fake.next != 0 {
			t.Fatal("explicit MOS consumed rows")
		}
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		return contract, layer
	}

	t.Run("explicit MOS inspects metadata without decoding valid rows", func(t *testing.T) {
		c, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), codec.FormatMOS)
		if !c.HasBounds || c.GeometryField != "geom" || c.ValidRows != 0 || c.ValidMOSRows != 0 {
			t.Fatalf("unexpected explicit MOS contract: %+v", c)
		}
	})

	for _, badFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("short auto MOS badFirst=%v", badFirst), func(t *testing.T) {
			rows := mosfixture.ValidRows()[:1]
			c, _ := probe(t, mosfixture.Columns(), rows, "")
			if !c.DetectsMOS() || !c.SampleComplete || c.ValidMOSRows != 1 {
				t.Fatalf("single MOS: %+v", c)
			}
			bad := mosfixture.RowsWithMalformed()[0]
			if badFirst {
				rows = append([][]interface{}{bad}, rows...)
			} else {
				rows = append(rows, bad)
			}
			c, _ = probe(t, mosfixture.Columns(), rows, "")
			if !c.DetectsMOS() || c.ValidMOSRows != 1 {
				t.Fatalf("mixed short MOS: %+v", c)
			}
		})
	}
	t.Run("three real MOS rows report the identical shared contract", func(t *testing.T) {
		contract, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), "")
		if contract != mosfixture.ExpectedContract() {
			t.Fatalf("contract %+v, expected identical fixture contract %+v", contract, mosfixture.ExpectedContract())
		}
	})

	t.Run("one malformed row plus three valid rows counts three", func(t *testing.T) {
		contract, _ := probe(t, mosfixture.Columns(), mosfixture.RowsWithMalformed(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (malformed rows skipped, never counted)", contract.ValidMOSRows)
		}
	})

	t.Run("system info row is skipped and never applied", func(t *testing.T) {
		contract, layer := probe(t, mosfixture.Columns(), mosfixture.RowsWithSystemInfo(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (system info row skipped)", contract.ValidMOSRows)
		}
		if layer.mosConfig != mosfixture.Config() {
			t.Fatalf("mosConfig mutated by probe: %+v", layer.mosConfig)
		}
	})

	t.Run("lower-case bounds columns persist actual names", func(t *testing.T) {
		contract, _ := probe(t, mosfixture.LowerCaseColumns(), mosfixture.ValidRows(), "")
		if contract != mosfixture.ExpectedLowerCaseContract() {
			t.Fatalf("contract %+v, expected %+v (actual result-column names)", contract, mosfixture.ExpectedLowerCaseContract())
		}
	})

	// audit part11 7.1.3 regression: native rows never count as MOS rows.
	t.Run("native geometry rows never count as MOS rows", func(t *testing.T) {
		contract, _ := probe(t, mosfixture.Columns(), mosfixture.NativeRows(), "")
		if contract.ValidMOSRows != 0 {
			t.Fatalf("ValidMOSRows = %d, expected 0 (native rows must not count as MOS)", contract.ValidMOSRows)
		}
	})

	// audit A04: the inference (auto) probe decode closure must fall back
	// to codec.DecodeMOS so MOS rows are recognized.
	t.Run("auto format probe falls back to MOS decode", func(t *testing.T) {
		contract, _ := probe(t, mosfixture.Columns(), mosfixture.ValidRows(), "")
		if contract.ValidMOSRows != 3 {
			t.Fatalf("ValidMOSRows = %d, expected 3 (auto decode must fall back to DecodeMOS)", contract.ValidMOSRows)
		}
	})
}

// adversarialProbeSQL pins audit R1: TWO zoom tokens plus a bbox token
// inside a SQL function call — the historical preparation replaced only the
// first !ZOOM! occurrence and neutralized !BBOX! to TRUE, which breaks
// ST_Intersects(geom, !BBOX!) at the parenthesis level.
const adversarialProbeSQL = `SELECT * FROM t WHERE min_zoom <= !ZOOM! AND max_zoom >= !ZOOM! AND geom && !BBOX! AND ST_Intersects(geom, !BBOX!) AND cx = !X! AND cy = !Y!`

// TestProbeSQLPreparationR1 asserts both PostGIS inspection probes follow
// shared permissive zoom/position handling (audit R1). Raw MOS bounds
// tokens become boolean predicates, while native PostGIS tokens remain
// typed geometry operands suitable for spatial function arguments. The
// query is wrapped in the shared InspectionSampleLimit sample window
// (docs/provider-contract.md).
func TestProbeSQLPreparationR1(t *testing.T) {
	newLayer := func() *Layer {
		return &Layer{
			name:       "probe_layer",
			srid:       3857,
			sql:        adversarialProbeSQL,
			idField:    "gid",
			geomField:  "geom",
			bboxFields: codec.DefaultBBoxFields(),
		}
	}

	check := func(t *testing.T, probeSQL string, native bool) {
		t.Helper()
		for _, tok := range []string{"!ZOOM!", "!BBOX!", "!X!", "!Y!"} {
			if strings.Contains(probeSQL, tok) {
				t.Errorf("token %s left in probe SQL: %s", tok, probeSQL)
			}
		}
		if !strings.Contains(probeSQL, "1=1") {
			t.Errorf("bbox token must neutralize to 1=1: %s", probeSQL)
		}
		if strings.Contains(probeSQL, "TRUE") {
			t.Errorf("bbox token must never neutralize to TRUE (breaks function arguments): %s", probeSQL)
		}
		wantOperand := "ST_Intersects(geom, 1=1)"
		if native {
			wantOperand = "ST_Intersects(geom, ST_MakeEnvelope("
		}
		if !strings.Contains(probeSQL, wantOperand) {
			t.Errorf("ST_Intersects(geom, !BBOX!) must probe as ST_Intersects(geom, 1=1): %s", probeSQL)
		}
		if !strings.Contains(probeSQL, fmt.Sprintf("LIMIT %v", codec.InspectionSampleLimit)) {
			t.Errorf("probe SQL must use the shared sample window LIMIT %v: %s", codec.InspectionSampleLimit, probeSQL)
		}
	}

	t.Run("MOS probe neutralizes every token occurrence", func(t *testing.T) {
		check(t, mosProbeSQL(newLayer()), false)
	})

	t.Run("native probe neutralizes every token occurrence", func(t *testing.T) {
		probeSQL, args, err := geomTypeProbeSQL(newLayer(), provider.Params{})
		if err != nil {
			t.Fatal(err)
		}
		if len(args) != 0 {
			t.Fatalf("args = %v, expected none without custom parameters", args)
		}
		check(t, probeSQL, true)
	})

	// The coordinator-verified live path: custom parameters substitute
	// with $N placeholders and their values stay bound as query arguments.
	t.Run("custom parameters keep live arguments", func(t *testing.T) {
		l := newLayer()
		l.sql = adversarialProbeSQL + " AND region = !REGION!"
		params := provider.Params{"!REGION!": {Token: "!REGION!", SQL: "?", Value: "west"}}
		probeSQL, args, err := geomTypeProbeSQL(l, params)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(probeSQL, "!REGION!") {
			t.Fatalf("parameter token must be substituted: %s", probeSQL)
		}
		if !strings.Contains(probeSQL, "$1") {
			t.Fatalf("parameter placeholder must be generated: %s", probeSQL)
		}
		if len(args) != 1 || args[0] != "west" {
			t.Fatalf("args = %v, expected live [west]", args)
		}
	})

	t.Run("MOS probe flow succeeds over fixture rows", func(t *testing.T) {
		l := newLayer()
		l.geometryFormat = codec.FormatMOS
		l.mosConfig = mosfixture.Config()
		probeSQL := mosProbeSQL(l)
		if err := inspectMOSGeomTypeRows(l, probeSQL, &probeFakeRows{columns: mosfixture.Columns(), rows: mosfixture.ValidRows()}); err != nil {
			t.Fatalf("probe over fixture rows: %v", err)
		}
		// DecodeMOS promotes the fixture WKB point to its multi-geometry
		// representation (MOS geometries decode as multi types).
		if _, ok := l.geomType.(geom.MultiPoint); !ok {
			t.Fatalf("geomType = %T, expected geom.MultiPoint", l.geomType)
		}
	})

	t.Run("native probe flow sniffs geometry type from sample rows", func(t *testing.T) {
		l := newLayer()
		l.sql = "SELECT ST_AsBinary(geom) AS geom, gid FROM t WHERE min_zoom <= !ZOOM! AND max_zoom >= !ZOOM!"
		probeSQL, _, err := geomTypeProbeSQL(l, provider.Params{})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(probeSQL, "!ZOOM!") {
			t.Fatalf("both !ZOOM! occurrences must be replaced: %s", probeSQL)
		}
		if err := inspectGeomTypeRows(l, probeSQL, &probeFakeRows{columns: []string{"st_geometrytype"}, rows: [][]any{{"ST_Point"}}}); err != nil {
			t.Fatalf("probe over sample rows: %v", err)
		}
		if _, ok := l.geomType.(geom.Point); !ok {
			t.Fatalf("geomType = %T, expected geom.Point", l.geomType)
		}
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
