package mysql

import (
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// TestReplaceTokensQuotesIdentifierTokens (audit P5-10) pins identifier
// quoting for !ID_FIELD!/!GEOM_FIELD! substitution: plain and qualified
// names are quoted
// via quoteIdentifier, values already wrapped in a complete quote pair pass
// through verbatim (backward compatibility), and hostile names cannot break
// out of the quoted identifier.
func TestReplaceTokensQuotesIdentifierTokens(t *testing.T) {
	tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
	extent := geom.NewExtent([2]float64{-10, -10}, [2]float64{10, 10})

	cases := []struct {
		name      string
		idField   string
		geomField string
		want      string
	}{
		{"plain name", "fid", "geom", "SELECT `fid`, `geom` FROM t"},
		{"qualified name", "t.fid", "db.t.geom", "SELECT `t`.`fid`, `db`.`t`.`geom` FROM t"},
		{"already backtick quoted", "`fid`", "`t`.`geom`", "SELECT `fid`, `t`.`geom` FROM t"},
		{"already double quoted", `"fid"`, `"geom"`, `SELECT "fid", "geom" FROM t`},
		{"unbalanced quotes", "`fid", `geom"`, "SELECT ```fid`, `geom\"` FROM t"},
		{"hostile input", "fid`,(SELECT 1)--", "geom`; DROP TABLE t; --", "SELECT `fid``,(SELECT 1)--`, `geom``; DROP TABLE t; --` FROM t"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			layer := &Layer{idFieldname: c.idField, geomFieldname: c.geomField}
			out, err := replaceTokens("SELECT !ID_FIELD!, !GEOM_FIELD! FROM t", layer, tile, extent)
			if err != nil {
				t.Fatalf("replaceTokens: %v", err)
			}
			if out != c.want {
				t.Fatalf("identifier tokens not quoted safely:\n got %q\nwant %q", out, c.want)
			}
		})
	}
}

// SQL-context-aware token substitution (UPSTREAM 2.6): tokens inside
// string literals, quoted identifiers and comments stay verbatim and are
// not uppercased; only SQL code-context tokens substitute.
func TestReplaceTokensProtectedContexts(t *testing.T) {
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)
	extent := geom.NewExtent([2]float64{-10, -10}, [2]float64{10, 10})
	layer := &Layer{geomFieldname: "geom", bboxFields: codec.DefaultBBoxFields()}

	sql := "SELECT '!zoom!', `!x!`, \"!y!\", '!z!'\n" +
		"FROM t WHERE z = !ZOOM! # !BBOX!\n" +
		"-- !id_field!"
	want := "SELECT '!zoom!', `!x!`, \"!y!\", '!z!'\n" +
		"FROM t WHERE z = 2 # !BBOX!\n" +
		"-- !id_field!"

	out, err := replaceTokens(sql, layer, tile, extent)
	if err != nil {
		t.Fatalf("replaceTokens: %v", err)
	}
	if out != want {
		t.Fatalf("protected contexts must stay verbatim:\n got %q\nwant %q", out, want)
	}
}

// TestReplaceTokensPreservesTrailingClauses covers the regression matrix row
// "WHERE ... AND !BBOX! ORDER BY ... -> ORDER BY preserved": the runtime
// token replacement must keep trailing clauses verbatim.
func TestReplaceTokensPreservesTrailingClauses(t *testing.T) {
	tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
	extent := geom.NewExtent([2]float64{-10, -10}, [2]float64{10, 10})
	layer := &Layer{geomFieldname: "geom", bboxFields: codec.DefaultBBoxFields()}
	out, err := replaceTokens("SELECT * FROM t WHERE kind = 'a' AND !BBOX! ORDER BY id DESC", layer, tile, extent)
	if err != nil {
		t.Fatalf("replaceTokens: %v", err)
	}
	if !strings.Contains(strings.ToUpper(out), "ORDER BY ID DESC") {
		t.Errorf("ORDER BY must be preserved: %q", out)
	}
	if strings.Contains(out, "!BBOX!") {
		t.Errorf("!BBOX! must be replaced: %q", out)
	}
}
