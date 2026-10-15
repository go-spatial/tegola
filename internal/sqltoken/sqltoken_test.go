package sqltoken_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/internal/sqltoken"
)

// wantSegment pairs a classification with the exact text it covers.
type wantSegment struct {
	kind sqltoken.Kind
	text string
}

// segs simplifies expected segment tables: every kind is quoted so tests
// can express boundaries unambiguously.
func checkSegments(t *testing.T, sql string, want []wantSegment) {
	t.Helper()
	got := sqltoken.Scan(sql)
	if len(got) != len(want) {
		t.Fatalf("Scan(%q) returned %d segments, want %d: %+v", sql, len(got), len(want), got)
	}
	var rebuilt strings.Builder
	for i, seg := range got {
		if seg.Kind != want[i].kind {
			t.Errorf("Scan(%q) segment %d kind = %v, want %v", sql, i, seg.Kind, want[i].kind)
		}
		text := sql[seg.Start:seg.End]
		if text != want[i].text {
			t.Errorf("Scan(%q) segment %d text = %q, want %q", sql, i, text, want[i].text)
		}
		if i > 0 && seg.Start != got[i-1].End {
			t.Errorf("Scan(%q) segment %d starts at %d, want %d (contiguous)", sql, i, seg.Start, got[i-1].End)
		}
		rebuilt.WriteString(text)
	}
	if rebuilt.String() != sql {
		t.Errorf("Scan(%q) segments rebuild to %q", sql, rebuilt.String())
	}
}

func TestScanProtectedContexts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sql  string
		want []wantSegment
	}{
		{
			name: "plain code",
			sql:  "SELECT !BBOX!",
			want: []wantSegment{{sqltoken.Code, "SELECT !BBOX!"}},
		},
		{
			name: "single-quoted string literal",
			sql:  "SELECT a, '!T!' FROM t",
			want: []wantSegment{
				{sqltoken.Code, "SELECT a, "},
				{sqltoken.StringLiteral, "'!T!'"},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "single-quoted string with doubled quote",
			sql:  "SELECT 'it''s !T!' FROM t",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.StringLiteral, "'it''s !T!'"},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "single-quoted string with backslash escape",
			sql:  `SELECT 'a\' !T! b' FROM t`,
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.StringLiteral, `'a\' !T! b'`},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "single-quoted string with escaped backslash",
			sql:  `SELECT 'a\\' , !T! FROM t`,
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.StringLiteral, `'a\\'`},
				{sqltoken.Code, " , !T! FROM t"},
			},
		},
		{
			name: "double-quoted identifier with doubled quote",
			sql:  `SELECT "a""!T!b" FROM t`,
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.QuotedIdentifier, `"a""!T!b"`},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "double-quoted identifier with backslash escape",
			sql:  `SELECT "a\" !T! b" FROM t`,
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.QuotedIdentifier, `"a\" !T! b"`},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "backtick identifier with doubled backtick",
			sql:  "SELECT `a``!T!b` FROM t",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.QuotedIdentifier, "`a``!T!b`"},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "bracket identifier with doubled closing bracket",
			sql:  "SELECT [a]]!T!b] FROM t",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.QuotedIdentifier, "[a]]!T!b]"},
				{sqltoken.Code, " FROM t"},
			},
		},
		{
			name: "dash line comment",
			sql:  "SELECT 1 -- !T!\n, 2",
			want: []wantSegment{
				{sqltoken.Code, "SELECT 1 "},
				{sqltoken.Comment, "-- !T!"},
				{sqltoken.Code, "\n, 2"},
			},
		},
		{
			name: "dash line comment without trailing space",
			sql:  "--!T!\nSELECT 1",
			want: []wantSegment{
				{sqltoken.Comment, "--!T!"},
				{sqltoken.Code, "\nSELECT 1"},
			},
		},
		{
			name: "hash line comment",
			sql:  "SELECT 1 # !T!\n, 2",
			want: []wantSegment{
				{sqltoken.Code, "SELECT 1 "},
				{sqltoken.Comment, "# !T!"},
				{sqltoken.Code, "\n, 2"},
			},
		},
		{
			name: "carriage return ends line comment",
			sql:  "SELECT 1 -- !T!\r\n, 2",
			want: []wantSegment{
				{sqltoken.Code, "SELECT 1 "},
				{sqltoken.Comment, "-- !T!"},
				{sqltoken.Code, "\r\n, 2"},
			},
		},
		{
			name: "block comment",
			sql:  "SELECT /* !T! */ 1",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.Comment, "/* !T! */"},
				{sqltoken.Code, " 1"},
			},
		},
		{
			name: "nested block comment",
			sql:  "SELECT /* a /* !T! */ b */ 1",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.Comment, "/* a /* !T! */ b */"},
				{sqltoken.Code, " 1"},
			},
		},
		{
			name: "unterminated block comment protects to end",
			sql:  "SELECT 1 /* !T!",
			want: []wantSegment{
				{sqltoken.Code, "SELECT 1 "},
				{sqltoken.Comment, "/* !T!"},
			},
		},
		{
			name: "unterminated string protects to end",
			sql:  "SELECT 1, '!T!",
			want: []wantSegment{
				{sqltoken.Code, "SELECT 1, "},
				{sqltoken.StringLiteral, "'!T!"},
			},
		},
		{
			name: "dollar-quoted string with tag",
			sql:  "SELECT $q$ !T! $q$, 1",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.DollarQuoted, "$q$ !T! $q$"},
				{sqltoken.Code, ", 1"},
			},
		},
		{
			name: "dollar-quoted string without tag",
			sql:  "SELECT $$ !T! $$, 1",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.DollarQuoted, "$$ !T! $$"},
				{sqltoken.Code, ", 1"},
			},
		},
		{
			name: "dollar-quoted string with underscore tag",
			sql:  "SELECT $my_tag_1$ !T! $my_tag_1$, 1",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.DollarQuoted, "$my_tag_1$ !T! $my_tag_1$"},
				{sqltoken.Code, ", 1"},
			},
		},
		{
			name: "placeholders are not dollar quotes",
			sql:  "SELECT $1, $2, !T!",
			want: []wantSegment{{sqltoken.Code, "SELECT $1, $2, !T!"}},
		},
		{
			name: "digit tag is not a dollar quote",
			sql:  "SELECT $1$ !T! $1$",
			want: []wantSegment{{sqltoken.Code, "SELECT $1$ !T! $1$"}},
		},
		{
			name: "unterminated dollar quote protects to end",
			sql:  "SELECT $$ !T! FROM t",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.DollarQuoted, "$$ !T! FROM t"},
			},
		},
		{
			name: "hash operator caveat is treated as comment",
			sql:  "SELECT doc #>> '{a}' = !T!",
			want: []wantSegment{
				{sqltoken.Code, "SELECT doc "},
				{sqltoken.Comment, "#>> '{a}' = !T!"},
			},
		},
		{
			name: "mixed contexts",
			sql:  "SELECT '!A!' /* !B! */ t.x -- !C!\nFROM \"!D!\" WHERE y = !E!",
			want: []wantSegment{
				{sqltoken.Code, "SELECT "},
				{sqltoken.StringLiteral, "'!A!'"},
				{sqltoken.Code, " "},
				{sqltoken.Comment, "/* !B! */"},
				{sqltoken.Code, " t.x "},
				{sqltoken.Comment, "-- !C!"},
				{sqltoken.Code, "\nFROM "},
				{sqltoken.QuotedIdentifier, `"!D!"`},
				{sqltoken.Code, " WHERE y = !E!"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkSegments(t, tc.sql, tc.want)
		})
	}
}

func TestCodeTokens(t *testing.T) {
	t.Parallel()

	sql := "SELECT '!A!' , !B! , \"!C!\" , !B! -- !D!\nFROM t WHERE !E_FIELD!"
	got := sqltoken.CodeTokens(sql)
	var texts []string
	for _, tok := range got {
		texts = append(texts, tok.Text)
		if sql[tok.Start:tok.End] != tok.Text {
			t.Errorf("token %q offsets [%d,%d) do not match text", tok.Text, tok.Start, tok.End)
		}
	}
	want := []string{"!B!", "!B!", "!E_FIELD!"}
	if !reflect.DeepEqual(texts, want) {
		t.Errorf("CodeTokens(%q) = %v, want %v", sql, texts, want)
	}
}

func TestMapTokens(t *testing.T) {
	t.Parallel()

	sql := "SELECT '!A!', !b!, !C! FROM \"!D!\" -- !e!\nWHERE x = $f$ !G! $f$"
	got := sqltoken.MapTokens(sql, strings.ToUpper)
	want := "SELECT '!A!', !B!, !C! FROM \"!D!\" -- !e!\nWHERE x = $f$ !G! $f$"
	if got != want {
		t.Errorf("MapTokens upper = %q, want %q", got, want)
	}

	// Replacing with the token identity keeps it verbatim everywhere.
	if got := sqltoken.MapTokens(sql, func(tok string) string { return tok }); got != sql {
		t.Errorf("MapTokens identity changed sql: %q", got)
	}
}

func TestReplaceToken(t *testing.T) {
	t.Parallel()

	sql := "SELECT !X!, !XY!, '!X!' FROM t WHERE a = !X!"
	got := sqltoken.ReplaceToken(sql, "!X!", "1")
	want := "SELECT 1, !XY!, '!X!' FROM t WHERE a = 1"
	if got != want {
		t.Errorf("ReplaceToken = %q, want %q", got, want)
	}
}

func TestStripTokens(t *testing.T) {
	t.Parallel()

	sql := "SELECT !A!, '!B!' FROM t -- !C!\nWHERE x = !D!"
	got := sqltoken.StripTokens(sql)
	want := "SELECT , '!B!' FROM t -- !C!\nWHERE x = "
	if got != want {
		t.Errorf("StripTokens = %q, want %q", got, want)
	}
}

func TestContainsToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sql   string
		t     []string
		want  bool
		wFold bool
	}{
		{
			name:  "code token",
			sql:   "SELECT !BBOX!",
			t:     []string{"!BBOX!"},
			want:  true,
			wFold: true,
		},
		{
			name:  "case differs",
			sql:   "SELECT !bbox!",
			t:     []string{"!BBOX!"},
			want:  false,
			wFold: true,
		},
		{
			name:  "token in string literal",
			sql:   "SELECT '!BBOX!'",
			t:     []string{"!BBOX!"},
			want:  false,
			wFold: false,
		},
		{
			name:  "token in comment",
			sql:   "SELECT 1 -- !BBOX!",
			t:     []string{"!BBOX!"},
			want:  false,
			wFold: false,
		},
		{
			name:  "token in quoted identifier",
			sql:   `SELECT "!BBOX!"`,
			t:     []string{"!BBOX!"},
			want:  false,
			wFold: false,
		},
		{
			name:  "token in dollar quote",
			sql:   "SELECT $$!BBOX!$$",
			t:     []string{"!BBOX!"},
			want:  false,
			wFold: false,
		},
		{
			name:  "any of several",
			sql:   "SELECT 1 WHERE !Z!",
			t:     []string{"!BBOX!", "!Z!"},
			want:  true,
			wFold: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sqltoken.ContainsToken(tc.sql, tc.t...); got != tc.want {
				t.Errorf("ContainsToken(%q, %v) = %v, want %v", tc.sql, tc.t, got, tc.want)
			}
			if got := sqltoken.ContainsTokenFold(tc.sql, tc.t...); got != tc.wFold {
				t.Errorf("ContainsTokenFold(%q, %v) = %v, want %v", tc.sql, tc.t, got, tc.wFold)
			}
		})
	}
}

func TestSpanHasProtectedToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sql   string
		start int
		end   int
		want  bool
	}{
		{
			name:  "code-only span with token",
			sql:   "!X! = 'west'",
			start: 0,
			end:   len("!X! = 'west'"),
			want:  false,
		},
		{
			name:  "span consuming a string literal with token",
			sql:   "'!X! = 1' = y",
			start: 0,
			end:   len("'!X! = 1'"),
			want:  true,
		},
		{
			name:  "span with token in comment",
			sql:   "a /* !T! */ = 1",
			start: 0,
			end:   len("a /* !T! */ = 1"),
			want:  true,
		},
		{
			name:  "span outside protected regions",
			sql:   "a = 1 -- !T!",
			start: 0,
			end:   len("a = 1"),
			want:  false,
		},
		{
			name:  "partial overlap of protected token",
			sql:   "x = '!T!'",
			start: 0,
			end:   len("x = '!T!'"),
			want:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sqltoken.SpanHasProtectedToken(tc.sql, tc.start, tc.end)
			if got != tc.want {
				t.Errorf("SpanHasProtectedToken(%q, %d, %d) = %v, want %v", tc.sql, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

func TestTokenRegexp(t *testing.T) {
	t.Parallel()

	for _, tok := range []string{"!BBOX!", "!BOX!", "!Z!", "!ID_FIELD!", "!x-1!"} {
		if !sqltoken.TokenRegexp.MatchString(tok) {
			t.Errorf("TokenRegexp should match %q", tok)
		}
	}
	for _, non := range []string{"!", "!!", "!a b!", "BBOX!", "!BBOX", "!a\nb!"} {
		if sqltoken.TokenRegexp.MatchString(non) {
			t.Errorf("TokenRegexp should not match %q", non)
		}
	}
}
