package hana

// Unit tests for audit part12 follow-up items N10 (getLayerFields rows
// lifecycle), N11 (isSrsRoundEarth placeholder and error propagation) and
// N12 (quoteIdentifier quoted-identifier parsing). All run against the
// contract stub driver; no live HANA server is required.

import (
	"database/sql/driver"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// TestParseQuotedIdent covers the quoted-identifier validation used by
// quoteIdentifier (audit N12).
func TestParseQuotedIdent(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`"a"`, true},
		{`"MyCol"`, true},
		{`"a""b"`, true},                // escaped quote inside
		{`""`, false},                   // audit P5-12: empty quoted identifier rejected
		{`""""`, true},                  // single literal quote as the name is non-empty
		{`"a""b""c""d"`, true},          // multiple escaped quotes
		{`"a"; DROP TABLE x;--`, false}, // trailing garbage after close
		{`"a"x`, false},                 // trailing garbage
		{`"abc`, false},                 // unterminated
		{`a"b`, false},                  // not quoted at all
		{``, false},                     // empty
		{`"`, false},                    // lone quote
		{`""a"`, false},                 // close then trailing quote
	}
	for _, c := range cases {
		if got := parseQuotedIdent(c.in); got != c.want {
			t.Errorf("parseQuotedIdent(%q) = %v, expected %v", c.in, got, c.want)
		}
	}
}

// TestQuoteIdentifier verifies that only fully valid quoted identifiers
// pass through and everything else is quoted as a literal identifier
// with doubled embedded quotes (audit N12).
func TestQuoteIdentifier(t *testing.T) {
	cases := []struct{ in, want string }{
		{"geom", `"geom"`},
		{`my"geom`, `"my""geom"`}, // embedded quote doubled
		{`MyCol`, `"MyCol"`},
		{`"MyCol"`, `"MyCol"`}, // valid quoted identifier passes through
		{`"a""b"`, `"a""b"`},   // valid with escapes passes through
		// audit P5-12: `""` is no longer a valid quoted identifier and is
		// quoted as the literal two-character name `""` instead.
		{`""`, `""""""`},
		// hostile inputs are neutralized as literal identifiers
		{`"a"; DROP TABLE x;--`, `"""a""; DROP TABLE x;--"`},
		{`"abc`, `"""abc"`},
		{`"a"x`, `"""a""x"`},
	}
	for _, c := range cases {
		if got := quoteIdentifier(c.in); got != c.want {
			t.Errorf("quoteIdentifier(%q) = %v, expected %v", c.in, got, c.want)
		}
	}
}

// TestValidateIdentName verifies that empty identifier names are
// rejected at registration with a clear error (audit P5-12): both the
// plain empty string and the empty quoted form `""` must be refused,
// while a quoted name whose content is a literal quote (`""""`) is a
// valid non-empty name.
func TestValidateIdentName(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", true},      // empty name
		{`""`, true},    // empty quoted identifier (audit P5-12)
		{`"`, false},    // not empty-content, parseQuotedIdent handles quoting
		{`x`, false},    // plain name
		{"geom", false}, // default geom field name
		{`""""`, false}, // one literal quote char is non-empty content
		{`"MyCol"`, false},
	}
	for _, c := range cases {
		err := validateIdentName(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("validateIdentName(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if c.wantErr && err != nil && !strings.Contains(err.Error(), "empty") {
			t.Errorf("validateIdentName(%q) error %q should name the empty-name problem", c.in, err.Error())
		}
	}
}

// TestIsSrsRoundEarth verifies the "?" placeholder probe, its error
// propagation, and the WGS84 shortcut (audit N11).
func TestIsSrsRoundEarth(t *testing.T) {
	t.Run("wgs84 shortcut does not query", func(t *testing.T) {
		db, queryLog := openContractStubLogged(t, []string{"ROUND_EARTH"}, nil)
		pool := &connectionPoolCollector{pool: db}
		got, err := isSrsRoundEarth(pool, tegola.WGS84)
		if err != nil || !got {
			t.Fatalf("isSrsRoundEarth(WGS84) = %v, %v, expected true, nil", got, err)
		}
		if len(*queryLog) != 0 {
			t.Errorf("expected no queries for the WGS84 shortcut, got: %v", *queryLog)
		}
	})

	t.Run("round earth true", func(t *testing.T) {
		db, queryLog := openContractStubLogged(t, []string{"ROUND_EARTH"}, [][]driver.Value{{true}})
		pool := &connectionPoolCollector{pool: db}
		got, err := isSrsRoundEarth(pool, 4004)
		if err != nil || !got {
			t.Fatalf("isSrsRoundEarth(4004) = %v, %v, expected true, nil", got, err)
		}
		if len(*queryLog) != 1 || !strings.Contains((*queryLog)[0], "SRS_ID = ?") {
			t.Errorf("expected the ? placeholder probe, got: %v", *queryLog)
		}
	})

	t.Run("planar false", func(t *testing.T) {
		db := openContractStub(t, []string{"ROUND_EARTH"}, [][]driver.Value{{false}})
		pool := &connectionPoolCollector{pool: db}
		got, err := isSrsRoundEarth(pool, 3857)
		if err != nil || got {
			t.Fatalf("isSrsRoundEarth(3857) = %v, %v, expected false, nil", got, err)
		}
	})

	t.Run("lookup error is returned", func(t *testing.T) {
		db := openContractStub(t, []string{"ROUND_EARTH"}, nil) // no row: unknown SRS
		pool := &connectionPoolCollector{pool: db}
		_, err := isSrsRoundEarth(pool, 999999)
		if err == nil {
			t.Fatal("expected an error for an unresolvable SRS, got nil")
		}
		if !strings.Contains(err.Error(), "round-earth lookup for srid 999999") {
			t.Errorf("error should name the lookup, got: %v", err)
		}
	})
}

// TestHasSrsPlanarEquivalent verifies error propagation of the paired
// SRS lookup (audit N11, tightly coupled to isSrsRoundEarth).
func TestHasSrsPlanarEquivalent(t *testing.T) {
	cases := []struct {
		name    string
		rows    [][]driver.Value
		want    bool
		wantErr bool
	}{
		{"planar exists", [][]driver.Value{{int64(1)}}, true, false},
		{"no planar", [][]driver.Value{{int64(0)}}, false, false},
		{"lookup error", nil, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := openContractStub(t, []string{"COUNT"}, c.rows)
			pool := &connectionPoolCollector{pool: db}
			got, err := hasSrsPlanarEquivalent(pool, 4326)
			if (err != nil) != c.wantErr {
				t.Fatalf("hasSrsPlanarEquivalent error = %v, expected error: %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("hasSrsPlanarEquivalent = %v, expected %v", got, c.want)
			}
		})
	}
}

// TestGetLayerFieldsClosesRows verifies the metadata probe releases its
// rows (audit N10).
func TestGetLayerFieldsClosesRows(t *testing.T) {
	// the stub reports BLOB column types: the geom column ("geom") passes
	// the field-type check, so the probe reaches its rows-iteration path.
	db, closes := openContractStubCounting(t, []string{"id", "geom", "name"}, [][]driver.Value{{int64(1), []byte{1}, "a"}})
	pool := &connectionPoolCollector{pool: db}
	l := Layer{
		name:           "probe_layer",
		idField:        "id",
		geomField:      "geom",
		geomType:       geom.Point{},
		srid:           3857,
		geometryFormat: "",
		bboxFields:     codec.DefaultBBoxFields(),
	}

	fields, err := getLayerFields(pool, &l, "SELECT * FROM probe_table")
	if err != nil {
		t.Fatalf("getLayerFields: %v", err)
	}
	if len(fields) != 3 {
		t.Errorf("expected 3 field descriptions, got %d: %+v", len(fields), fields)
	}
	if n := atomic.LoadInt32(closes); n != 1 {
		t.Errorf("expected the probe rows to be closed exactly once, got %d closes", n)
	}
}
