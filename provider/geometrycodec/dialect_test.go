package geometrycodec_test

import (
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"strings"
	"testing"
)

func TestDialectProbePreparation(t *testing.T) {
	for _, tc := range []struct {
		dialect   codec.SQLDialect
		sql, want string
	}{
		{codec.PostgreSQL, `SELECT ARRAY[!X!], j #>> '{a}' FROM t WHERE z=!ZOOM! AND !BBOX!`, `SELECT ARRAY[0], j #>> '{a}' FROM t WHERE z IN (0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24) AND 1=1`},
		{codec.SQLite, `SELECT '\', !X! FROM t WHERE !BBOX!`, `SELECT '\', 0 FROM t WHERE 1=1`},
		{codec.MySQL, "SELECT 1--!X! # !BBOX!\nFROM t WHERE !BBOX!", "SELECT 1--0 # !BBOX!\nFROM t WHERE 1=1"},
	} {
		got := tc.dialect.PrepareProbeSQL(tc.sql, "geom", "id", "")
		if got != tc.want {
			t.Fatalf("got %q; want %q", got, tc.want)
		}
		if err := tc.dialect.RequireBBoxCustomSQL("layer", true, tc.sql, "!BBOX!"); err != nil {
			t.Fatal(err)
		}
	}
	if err := codec.PostgreSQL.RequireBBoxCustomSQL("layer", true, `SELECT j #> '{a}' FROM t WHERE !BBOX!`, "!BBOX!"); err != nil {
		t.Fatal(err)
	}
	if err := codec.MySQL.RequireBBoxCustomSQL("layer", true, `SELECT 1 # !BBOX!`, "!BBOX!"); err == nil {
		t.Fatal("comment must not satisfy contract")
	}
}

func TestProbeWrapperTerminatesLineComment(t *testing.T) {
	for _, wrap := range []func(string) string{codec.WrapProbeSQL, codec.WrapProbeSQLTopStyle} {
		got := wrap("SELECT 1 -- trailing comment\n")
		if !strings.Contains(got, "-- trailing comment\n) AS") {
			t.Fatalf("wrapper swallowed by comment: %q", got)
		}
	}
}
