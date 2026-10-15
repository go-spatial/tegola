package geometrycodec

import (
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
)

func TestResolveBBoxTable(t *testing.T) {
	for _, layer := range []dict.Dicter{nil, dict.Dict{}} {
		if got, err := ResolveBBoxTable(layer, "chambers"); got != "" || err != nil {
			t.Fatalf("absent qualifier = %q, %v", got, err)
		}
	}
	for _, table := range []string{"c", "chambers", "moek.chambers", " _source.source_2 "} {
		got, err := ResolveBBoxTable(dict.Dict{ConfigKeyBBoxTable: table}, "chambers")
		if err != nil || got != strings.TrimSpace(table) {
			t.Errorf("qualifier %q = %q, %v", table, got, err)
		}
	}
	for _, value := range []interface{}{
		"", " ", ".table", "schema.", "schema..table", "db.schema.table",
		"table; DROP TABLE chambers", "t -- comment", "t/*comment*/", "t'", "`t`", `"t"`,
		"schema. table", "1table", "t/other", 42,
	} {
		_, err := ResolveBBoxTable(dict.Dict{ConfigKeyBBoxTable: value}, "chambers")
		if err == nil || !strings.Contains(err.Error(), "chambers") || !strings.Contains(err.Error(), "bbox_table") {
			t.Errorf("invalid qualifier %v: expected contextual error, got %v", value, err)
		}
	}
}

func TestBoundsQuotePredicate(t *testing.T) {
	for _, delimiter := range []string{"`", `"`, ""} {
		t.Run("quote="+delimiter, func(t *testing.T) {
			quote := func(name string) string { return delimiter + name + delimiter }
			fields := DefaultBBoxFields()
			for _, table := range []string{"", "c", "moek.chambers"} {
				qualifier := ""
				if table != "" {
					parts := strings.Split(table, ".")
					for _, part := range parts {
						qualifier += quote(part) + "."
					}
				}
				got, err := BuildBoundsPredicate(
					fields,
					geom.NewExtent(geom.Point{1, 2}, geom.Point{3, 4}),
					BoundsMOSRaw,
					MOSConfig{Precision: 2, UnitFactor: 1},
					BoundsQuote(table, quote),
				)
				want := qualifier + quote("MAXX") + " >= 100 AND " + qualifier + quote("MINX") +
					" <= 300 AND " + qualifier + quote("MAXY") + " >= 200 AND " + qualifier + quote("MINY") + " <= 400"
				if err != nil || got != want {
					t.Fatalf("table %q: got %q, %v; want %q", table, got, err, want)
				}
				if fields != DefaultBBoxFields() || !fields.IsBBoxField("MINX") || fields.IsBBoxField("c.MINX") {
					t.Fatal("qualifier changed the unqualified result-column contract")
				}
			}
		})
	}
	if BoundsQuote("", nil) != nil {
		t.Fatal("empty qualifier must preserve nil quoting policy")
	}
	if got := BoundsQuote("schema.table", nil)("MINX"); got != "schema.table.MINX" {
		t.Fatalf("nil quoting policy: %q", got)
	}
}
