package postgis

import (
	"strings"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
)

// SQL-context-aware token substitution (UPSTREAM 2.6): tokens inside
// string literals (including PostgreSQL dollar-quoted strings), quoted
// identifiers and comments stay verbatim and are not uppercased; only SQL
// code-context tokens substitute.
func TestReplaceTokensProtectedContexts(t *testing.T) {
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)
	lyr := &Layer{name: "t", srid: tegola.WebMercator}

	sql := "SELECT '!zoom!', \"!x!\", !ZOOM! -- !BBOX!\n" +
		"FROM t WHERE a = $tag$!ID_FIELD!$tag$ AND b = $$!GEOM_TYPE!$$ AND c = 'it''s \\!y!'"
	want := "SELECT '!zoom!', \"!x!\", 2 -- !BBOX!\n" +
		"FROM t WHERE a = $tag$!ID_FIELD!$tag$ AND b = $$!GEOM_TYPE!$$ AND c = 'it''s \\!y!'"

	out, err := replaceTokens(sql, lyr, tile, false)
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
	lyr := &Layer{name: "t", srid: 3857}
	out, err := replaceTokens("SELECT * FROM t WHERE kind = 'a' AND !BBOX! ORDER BY id DESC", lyr, tile, false)
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
