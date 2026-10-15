package provider

import "testing"

func TestReplaceParamsDialect(t *testing.T) {
	for _, tc := range []struct {
		dialect   SQLDialect
		sql, want string
	}{
		{SQLDialectPostgreSQL, `SELECT ARRAY[!P!], j #> '{a}', '\', !P!, E'!P!'`, `SELECT ARRAY[$1], j #> '{a}', '\', $1, E'!P!'`},
		{SQLDialectSQLite, `SELECT '\', !P!, [!P!]`, `SELECT '\', $1, [!P!]`},
		{SQLDialectMySQL, "SELECT 1--!P! # !P!\n, !P!", "SELECT 1--$1 # !P!\n, $1"},
	} {
		args := []interface{}{}
		params := Params{"!P!": {SQL: "?", Value: 17}}
		got := params.ReplaceParamsWithDialect(tc.sql, &args, tc.dialect)
		if got != tc.want || len(args) != 1 || args[0] != 17 {
			t.Fatalf("got %q args=%v; want %q", got, args, tc.want)
		}
	}
}
