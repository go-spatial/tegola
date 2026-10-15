package hana

import (
	"strings"
	"testing"
)

// audit P5-3: quote-aware qualified identifier parsing.
//
// The old implementation split identifiers with strings.Split(name, ".")
// which (a) shreds quoted identifiers that contain dots
// ("my.schema"."my.table" -> "my" and "table") and (b) splits hostile
// names into injection-shaped fragments. parseIdentParts treats quoted
// parts as atomic and rejects malformed names.

func TestParseIdentParts(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"unqualified", `mytable`, []string{"mytable"}, false},
		{"qualified", `schema.table`, []string{"schema", "table"}, false},
		{"dotted quoted parts", `"my.schema"."my.table"`, []string{"my.schema", "my.table"}, false},
		{"escaped quote in part", `"a""b".t`, []string{`a"b`, "t"}, false},
		{"hostile bare part stays one ident", `a;b--.t`, []string{"a;b--", "t"}, false},
		{"hostile quoted part stays one ident", `"select * from x;--".t`, []string{"select * from x;--", "t"}, false},
		{"empty", ``, nil, true},
		{"empty quoted part", `""`, nil, true},
		{"trailing dot", `a.`, nil, true},
		{"leading dot", `.a`, nil, true},
		{"empty middle part", `a..b`, nil, true},
		{"too many parts", `a.b.c`, nil, true},
		{"unterminated quote", `"abc`, nil, true},
		{"content after closing quote", `"a"x`, nil, true},
		{"quote inside bare part", `a"b`, nil, true},
		{"more after quoted empty", `""a"`, nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIdentParts(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseIdentParts(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIdentParts(%q) error: %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseIdentParts(%q) = %q, want %q", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseIdentParts(%q) = %q, want %q", tc.in, got, tc.want)
				}
			}
		})
	}
}

func TestSplitQualifiedTableName(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantSchema string
		wantTable  string
		wantErr    bool
	}{
		{"unqualified", `mytable`, "", "mytable", false},
		{"qualified", `schema.table`, "schema", "table", false},
		// regression: the old strings.Split+strings.Trim derived
		// schema "my" and table "table" here
		{"dotted quoted parts", `"my.schema"."my.table"`, "my.schema", "my.table", false},
		{"escaped quote in part", `"a""b".t`, `a"b`, "t", false},
		{"single quoted dotted table", `"my.table"`, "", "my.table", false},
		{"too many parts", `a.b.c`, "", "", true},
		{"empty quoted part", `""`, "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema, table, err := splitQualifiedTableName(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("splitQualifiedTableName(%q) = (%q, %q), want error", tc.in, schema, table)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitQualifiedTableName(%q) error: %v", tc.in, err)
			}
			if schema != tc.wantSchema || table != tc.wantTable {
				t.Fatalf("splitQualifiedTableName(%q) = (%q, %q), want (%q, %q)",
					tc.in, schema, table, tc.wantSchema, tc.wantTable)
			}
		})
	}
}

func TestValidateTableName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"unqualified", `mytable`, false},
		{"qualified", `schema.table`, false},
		{"dotted quoted parts", `"my.schema"."my.table"`, false},
		{"hostile part quoted later", `a;b--.t`, false},
		{"subquery passthrough", `(SELECT * FROM tbl) x`, false},
		{"empty", ``, true},
		{"empty quoted part", `""`, true},
		{"too many parts", `a.b.c`, true},
		{"unterminated quote", `"abc`, true},
		{"quote inside bare part", `a"b`, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTableName(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("validateTableName(%q) = nil, want error", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTableName(%q) error: %v", tc.in, err)
			}
		})
	}
}

// TestQuoteTableNameQualified proves quoteTableName no longer splits on
// dots inside quoted identifiers and never emits injection-shaped
// fragments (audit P5-3).
func TestQuoteTableNameQualified(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"unqualified", `mytable`, `"mytable"`},
		{"qualified", `schema.table`, `"schema"."table"`},
		// regression: pre-fix strings.Split shredded this into the
		// quoted fragments "my" / "schema""."my / table"
		{"dotted quoted parts", `"my.schema"."my.table"`, `"my.schema"."my.table"`},
		{"escaped quote in part", `"a""b".t`, `"a""b"."t"`},
		{"hostile bare part", `a;b--.t`, `"a;b--"."t"`},
		{"subquery passthrough", `(SELECT * FROM tbl) x`, `(SELECT * FROM tbl) x`},
		// parse error -> whole name quoted as one identifier (safe)
		{"unparseable names quoted whole", `a.b.c`, `"a.b.c"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteTableName(tc.in)
			if got != tc.want {
				t.Fatalf("quoteTableName(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// a quoted table reference must never contain a
			// statement terminator outside of quotes
			if stripped := stripQuoted(got); strings.Contains(stripped, ";") {
				t.Fatalf("quoteTableName(%q) = %q leaks a bare ';'", tc.in, got)
			}
		})
	}
}

// stripQuoted removes double-quoted segments (with "" escapes) so tests
// can assert that no unquoted metacharacters survive quoting.
func stripQuoted(s string) string {
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '"' {
				if i+1 < len(s) && s[i+1] == '"' {
					i++
					continue
				}
				inQuote = false
			}
			continue
		}
		if c == '"' {
			inQuote = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
