package hana

import (
	"strings"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
)

// audit P5-8: values interpolated into SQL string literals must escape
// single quotes (' -> ''); audit P5-10 contract: identifier tokens are
// quoted with pass-through for already-quoted values.

func TestEscapeSQLStringLiteral(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"we'ird", "we''ird"},
		{"a'b'c", "a''b''c"},
		{"''", "''''"},
		{`quote"double`, `quote"double`},
	}

	for _, tc := range tests {
		if got := escapeSQLStringLiteral(tc.in); got != tc.want {
			t.Fatalf("escapeSQLStringLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestGenMVTSQLStringLiteralEscaping proves the ST_AsMVT named-argument
// string literals (layer_name / geom_name / feature_id_name) are quote
// escaped on every generation branch (audit P5-8).
func TestGenMVTSQLStringLiteralEscaping(t *testing.T) {
	tests := []struct {
		name         string
		layer        Layer
		fields       []string
		wantContains []string
	}{
		{
			name:   "no attribute fields",
			layer:  Layer{name: "we'ird", geomField: "g'geom", srid: tegola.WebMercator},
			fields: []string{"g'geom"},
			wantContains: []string{
				`layer_name => 'we''ird'`,
				`geom_name => 'g''geom'`,
			},
		},
		{
			name:   "attribute fields without id",
			layer:  Layer{name: "we'ird", geomField: "g'geom", srid: tegola.WebMercator},
			fields: []string{"a", "g'geom"},
			wantContains: []string{
				`layer_name => 'we''ird'`,
				`geom_name => 'g''geom'`,
			},
		},
		{
			name:   "attribute fields with id",
			layer:  Layer{name: "we'ird", geomField: "g'geom", idField: "i'd", srid: tegola.WebMercator},
			fields: []string{"i'd", "g'geom"},
			wantContains: []string{
				`layer_name => 'we''ird'`,
				`geom_name => 'g''geom'`,
				`feature_id_name => 'i''d'`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.layer
			sql, err := genMVTSQL(&l, tc.fields, 4096, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(sql, want) {
					t.Fatalf("generated SQL missing %q:\n%v", want, sql)
				}
			}
			// the unescaped literal must not appear
			if strings.Contains(sql, `layer_name => 'we'ird'`) {
				t.Fatalf("generated SQL contains an unescaped layer_name literal:\n%v", sql)
			}
		})
	}
}

// TestSQLStringLiteral pins the audit P5-8 helper contract: values are
// wrapped as single-quoted literals with inner single quotes doubled.
func TestSQLStringLiteral(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "'plain'"},
		{"we'ird", "'we''ird'"},
		{"'", "''''"},
		{"", "''"},
	}
	for _, tc := range tests {
		if got := sqlStringLiteral(tc.in); got != tc.want {
			t.Fatalf("sqlStringLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestQuoteTokenIdentifier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "geom", `"geom"`},
		{"already quoted", `"geom"`, `"geom"`},
		{"already quoted with escape", `"a""b"`, `"a""b"`},
		{"backtick quoted passes through", "`geom`", "`geom`"},
		{"empty no-id sentinel", "", ""},
		{"hostile", `x" ; DROP`, `"x"" ; DROP"`},
		{"single quote inside", "we'ird", `"we'ird"`},
		{"single-quoted value is not identifier quoting", "'geom'", `"'geom'"`},
		{"qualified name quoted per part", "s.t.c", `"s"."t"."c"`},
		{"qualified quoted parts pass through", `"my.schema"."my.table"`, `"my.schema"."my.table"`},
		{"hostile qualified value never splits raw", "a;b--.t", `"a;b--"."t"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := quoteTokenIdentifier(tc.in); got != tc.want {
				t.Fatalf("quoteTokenIdentifier(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestReplaceTokensQuotedIdents proves !ID_FIELD!/!GEOM_FIELD! token
// substitution quotes identifiers (P5-10 contract) with pass-through
// for already-quoted names.
func TestReplaceTokensQuotedIdents(t *testing.T) {
	tests := []struct {
		name     string
		idField  string
		geomF    string
		sql      string
		expected string
	}{
		{
			name:     "plain names get quoted",
			idField:  "id",
			geomF:    "geom",
			sql:      "SELECT !ID_FIELD! FROM x WHERE !BBOX!",
			expected: `SELECT "id" FROM x WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		{
			name:     "already quoted passes through",
			idField:  `"my.id"`,
			geomF:    "geom",
			sql:      "SELECT !ID_FIELD! FROM x WHERE !BBOX!",
			expected: `SELECT "my.id" FROM x WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		{
			name:     "hostile name quoted as one identifier",
			idField:  `x" ; DROP`,
			geomF:    "geom",
			sql:      "SELECT !ID_FIELD! FROM x WHERE !BBOX!",
			expected: `SELECT "x"" ; DROP" FROM x WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := Layer{srid: tegola.WebMercator, geomField: tc.geomF, idField: tc.idField}
			tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)
			sql, err := replaceTokens(4, tc.sql, &l, l.GeomType(), l.SRID(), tile, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.expected {
				t.Fatalf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expected, sql)
			}
		})
	}
}
