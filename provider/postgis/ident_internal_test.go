package postgis

import (
	"strings"
	"testing"

	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

// TestSplitTableName covers the audit N7 identifier handling: quoted
// identifiers are unquoted correctly, dots and quotes inside names survive,
// and injection-shaped input is rejected instead of being split into
// garbage schema/table pairs.
func TestSplitTableName(t *testing.T) {
	type tcase struct {
		name       string
		in         string
		wantSchema string
		wantTable  string
		wantErr    bool
	}

	tests := []tcase{
		{
			name:       "unqualified bare name defaults to public",
			in:         "roads",
			wantSchema: "public",
			wantTable:  "roads",
		},
		{
			name:       "qualified bare name",
			in:         "gis.roads",
			wantSchema: "gis",
			wantTable:  "roads",
		},
		{
			name:       "legacy first-dot split for unquoted multi-dot input",
			in:         "a.b.c",
			wantSchema: "a",
			wantTable:  "b.c",
		},
		{
			name:       "quoted parts may contain dots",
			in:         `"my.schema"."my.table"`,
			wantSchema: "my.schema",
			wantTable:  "my.table",
		},
		{
			name:       "escaped quotes are unquoted",
			in:         `myschema."we""ird"`,
			wantSchema: "myschema",
			wantTable:  `we"ird`,
		},
		{
			name:       "quoted part may contain semicolons and quotes-as-data",
			in:         `public."a; DROP TABLE x;--"`,
			wantSchema: "public",
			wantTable:  `a; DROP TABLE x;--`,
		},
		{
			name:       "single quoted part",
			in:         `"justtable"`,
			wantSchema: "public",
			wantTable:  "justtable",
		},
		{
			name:    "trailing garbage after closing quote rejected",
			in:      `"a"; DROP TABLE x;--`,
			wantErr: true,
		},
		{
			name:    "quote inside bare part rejected",
			in:      `a"; DROP TABLE x;--`,
			wantErr: true,
		},
		{
			name:    "unterminated quote rejected",
			in:      `"unterminated`,
			wantErr: true,
		},
		{
			name:    "empty identifier rejected",
			in:      `""`,
			wantErr: true,
		},
		{
			name:    "trailing dot rejected",
			in:      `"a".`,
			wantErr: true,
		},
		{
			name:    "more than two parts rejected",
			in:      `"a"."b"."c"`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema, table, err := splitTableName(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got schema=%q table=%q", tc.in, schema, table)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if schema != tc.wantSchema || table != tc.wantTable {
				t.Errorf("splitTableName(%q) = (%q, %q), want (%q, %q)",
					tc.in, schema, table, tc.wantSchema, tc.wantTable)
			}
		})
	}
}

// TestFindSRIDQueryNoInterpolation pins the audit N7 contract for the
// source-SRID auto-detect: names travel as query parameters and never enter
// the SQL text.
func TestFindSRIDQueryNoInterpolation(t *testing.T) {
	schema := `evil'; DROP TABLE x;--`
	table := `we"ird.table`
	geom := `col"';--`

	query, args := findSRIDQuery(schema, table, geom)
	if query != `SELECT Find_SRID($1, $2, $3)` {
		t.Fatalf("unexpected query text: %q", query)
	}
	if len(args) != 3 || args[0] != schema || args[1] != table || args[2] != geom {
		t.Fatalf("adversarial names must travel verbatim as args, got %#v", args)
	}
	for _, a := range args {
		if s, ok := a.(string); ok && strings.Contains(query, s) {
			t.Fatalf("value %q leaked into the SQL text %q", s, query)
		}
	}
}

// TestMapplGISSystemInfoSQLQuotesIdentifiers pins that the MapplGIS
// system-info fetch quotes every identifier with doubled embedded quotes
// (audit N7), so adversarial schema/table names stay data, not SQL.
func TestMapplGISSystemInfoSQLQuotesIdentifiers(t *testing.T) {
	got := mapplGISSystemInfoSQL("public", "roads")
	want := `SELECT "LINE" FROM "public"."roads" WHERE "OKEY" = 1 AND "LINE" IS NOT NULL LIMIT 1`
	if got != want {
		t.Fatalf("unexpected SQL:\n got %q\nwant %q", got, want)
	}

	adv := mapplGISSystemInfoSQL(`we"ird`, `a"; DROP TABLE x;--`)
	if !strings.Contains(adv, `"we""ird"."a""; DROP TABLE x;--"`) {
		t.Errorf("identifiers must be quoted with doubled quotes, got %q", adv)
	}
	if strings.Contains(adv, `a"; DROP`) {
		t.Errorf("raw adversarial name must not appear unquoted, got %q", adv)
	}
}

// TestPgQuoteIdentDoublesEmbeddedQuotes documents that pgQuoteIdent already
// escapes correctly (audit claim that it does not double internal quotes is
// false); the non-doubling quoting was genSQL's ad-hoc "%v" wrapping.
func TestPgQuoteIdentDoublesEmbeddedQuotes(t *testing.T) {
	for in, want := range map[string]string{
		"plain":   `"plain"`,
		`we"ird`:  `"we""ird"`,
		`""`:      `""""""`,
		`a"."b"b`: `"a"".""b""b"`,
	} {
		if got := pgQuoteIdent(in); got != want {
			t.Errorf("pgQuoteIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestGenSQLQuotedIdentifiersEscaped covers the actual audit N7 quoting
// site: genSQL must double embedded quotes in field and geometry column
// identifiers across all three SQL templates.
func TestGenSQLQuotedIdentifiersEscaped(t *testing.T) {
	type tcase struct {
		name           string
		geometryFormat string
		srid           uint64
		flds           []string
		expectedSQL    string
	}

	tests := []tcase{
		{
			name:           "native format",
			geometryFormat: "",
			srid:           3857,
			flds:           []string{`i"d`, `we"ird`},
			expectedSQL:    `SELECT "i""d", ST_AsBinary("we""ird") AS "we""ird", "i""d" FROM tbl WHERE "we""ird" && !BBOX!`,
		},
		{
			name:           "raw mos format",
			geometryFormat: geometrycodec.FormatMOS,
			srid:           3857,
			flds:           []string{`i"d`, `we"ird`},
			expectedSQL:    `SELECT "i""d", "we""ird" AS "we""ird", "i""d" FROM tbl WHERE "we""ird" IS NOT NULL`,
		},
		{
			name:           "synthetic SRID keeps SRID 0 comparisons",
			geometryFormat: "",
			srid:           uint64(basic.SyntheticSRIDMin),
			flds:           []string{`i"d`, `we"ird`},
			expectedSQL:    `SELECT "i""d", ST_AsBinary("we""ird") AS "we""ird", "i""d" FROM tbl WHERE ST_SetSRID("we""ird",0) && !BBOX!`,
		},
		{
			name:           "geometry column outside the field list",
			geometryFormat: "",
			srid:           3857,
			flds:           []string{`i"d`},
			expectedSQL:    `SELECT "i""d", ST_AsBinary("we""ird") AS "we""ird", "i""d" FROM tbl WHERE "we""ird" && !BBOX!`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := &Layer{
				name:           "layer",
				geomField:      `we"ird`,
				idField:        `i"d`,
				srid:           tc.srid,
				geometryFormat: tc.geometryFormat,
			}
			sql, err := genSQL(l, nil, "tbl", tc.flds, false, ProviderType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.expectedSQL {
				t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expectedSQL, sql)
			}
		})
	}
}
