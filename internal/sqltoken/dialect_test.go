package sqltoken_test

import (
	"github.com/alexeydott/tegola/internal/sqltoken"
	"strings"
	"testing"
)

func TestDialectExecutableTokens(t *testing.T) {
	cases := []struct {
		name      string
		dialect   sqltoken.Dialect
		sql, want string
	}{
		{"PG arrays and JSON", sqltoken.PostgreSQL, `SELECT ARRAY[!X!], arr[!X!], j #>> '{a}' FROM t WHERE z = !X!`, `SELECT ARRAY[7], arr[7], j #>> '{a}' FROM t WHERE z = 7`},
		{"PG ordinary backslash", sqltoken.PostgreSQL, `SELECT '\', !X!, "a\", !X!`, `SELECT '\', 7, "a\", 7`},
		{"PG escape string", sqltoken.PostgreSQL, `SELECT E'a\' !X! b', !X!, e'!X!'`, `SELECT E'a\' !X! b', 7, e'!X!'`},
		{"PG E identifier boundary", sqltoken.PostgreSQL, `SELECT name'\', !X!`, `SELECT name'\', 7`},
		{"PG dollar identifier boundary", sqltoken.PostgreSQL, `SELECT col$tag$, !X!, $tag$!X!$tag$`, `SELECT col$tag$, 7, $tag$!X!$tag$`},
		{"PG nested comments", sqltoken.PostgreSQL, `SELECT /* outer /* inner */ !X! */ !X!`, `SELECT /* outer /* inner */ !X! */ 7`},
		{"SQLite backslash and identifiers", sqltoken.SQLite, "SELECT '\\', !X!, [!X!], `!X!`, \"!X!\"", "SELECT '\\', 7, [!X!], `!X!`, \"!X!\""},
		{"SQLite no nested comment", sqltoken.SQLite, `SELECT /* outer /* inner */ !X!`, `SELECT /* outer /* inner */ 7`},
		{"MySQL hash comment", sqltoken.MySQL, "SELECT !X! # !X!\n, !X!", "SELECT 7 # !X!\n, 7"},
		{"MySQL dash expression", sqltoken.MySQL, `SELECT 1--!X!, !X!`, `SELECT 1--7, 7`},
		{"MySQL dash comment", sqltoken.MySQL, "SELECT 1-- !X!\n, !X!", "SELECT 1-- !X!\n, 7"},
		{"MySQL escape string", sqltoken.MySQL, `SELECT 'a\' !X! b', !X!`, `SELECT 'a\' !X! b', 7`},
		{"MySQL backtick no slash escape", sqltoken.MySQL, "SELECT `a\\`, !X!, `!X!`", "SELECT `a\\`, 7, `!X!`"},
		{"HANA backslash", sqltoken.HANA, `SELECT '\', !X!, "a\", !X!`, `SELECT '\', 7, "a\", 7`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.dialect.ReplaceToken(tc.sql, "!X!", "7")
			if got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
			// Detection and stripping must see precisely the same executable sites.
			if !tc.dialect.ContainsToken(tc.sql, "!X!") {
				t.Fatal("token detection disagrees with replacement")
			}
			if stripped := tc.dialect.StripTokens(tc.sql); stripped != strings.ReplaceAll(tc.want, "7", "") {
				t.Fatalf("strip disagrees: %q", stripped)
			}
		})
	}
}
