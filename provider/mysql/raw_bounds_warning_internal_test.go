package mysql

import (
	"strings"
	"testing"

	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// P6-19 regression: raw geometry formats (wkb/wkt/mos) whose per-tile query
// has no bounds-columns-backed server-side predicate force a full-table scan
// per tile request. The registration path must warn for exactly those layers.
//
// Red for this seam-introduction test is a compile failure (the helpers do
// not exist before the fix).

func TestRawGeometryBoundsWarning(t *testing.T) {
	type tc struct {
		name                string
		layerName           string
		geometryFormat      string
		boundsPredicateUsed bool
		wantWarn            bool
	}

	tests := []tc{
		{
			name:           "raw wkb warns",
			layerName:      "roads",
			geometryFormat: codec.FormatWKB,
			wantWarn:       true,
		},
		{
			name:           "raw wkt warns",
			layerName:      "lakes",
			geometryFormat: codec.FormatWKT,
			wantWarn:       true,
		},
		{
			name:                "mos with bounds-columns predicate is silent",
			layerName:           "mappl",
			geometryFormat:      codec.FormatMOS,
			boundsPredicateUsed: true,
			wantWarn:            false,
		},
		{
			name:           "native geometry format never warns",
			layerName:      "native",
			geometryFormat: "",
			wantWarn:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := rawGeometryBoundsWarning(tt.layerName, tt.geometryFormat, tt.boundsPredicateUsed)
			if tt.wantWarn {
				if msg == "" {
					t.Fatalf("expected a registration warning for layer %q format %q", tt.layerName, tt.geometryFormat)
				}
				if !strings.Contains(msg, tt.layerName) {
					t.Errorf("warning must name the layer %q; got %q", tt.layerName, msg)
				}
				if !strings.Contains(msg, "tile request") {
					t.Errorf("warning must explain the per-tile cost; got %q", msg)
				}
				if !strings.Contains(msg, "bbox_minx_fieldname") {
					t.Errorf("warning must point at the bounds-column setup; got %q", msg)
				}
				return
			}
			if msg != "" {
				t.Fatalf("unexpected warning for layer %q format %q: %q", tt.layerName, tt.geometryFormat, msg)
			}
		})
	}
}

func TestRawGeometryBoundsWarningsRegistrationSweep(t *testing.T) {
	layers := map[string]Layer{
		"wkb-table": {name: "wkb-table", geometryFormat: codec.FormatWKB, tablename: "t", geomFieldname: "geom"},
		"wkt-sql":   {name: "wkt-sql", geometryFormat: codec.FormatWKT, sql: "SELECT id, geom FROM t"},
		"mos-table": {name: "mos-table", geometryFormat: codec.FormatMOS, tablename: "t", geomFieldname: "geom"},
		"mos-sql": {
			name:           "mos-sql",
			geometryFormat: codec.FormatMOS,
			sql:            "SELECT id, geom, minx, maxx FROM t WHERE minx <= !BBOX! AND maxx >= !BBOX!",
		},
		"native-table": {name: "native-table", tablename: "t", geomFieldname: "geom"},
	}

	msgs := rawGeometryBoundsWarnings(layers)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 warnings (wkb-table, wkt-sql), got %d: %v", len(msgs), msgs)
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"wkb-table", "wkt-sql"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings must name layer %q; got %v", want, msgs)
		}
	}
	for _, unwanted := range []string{"mos-table", "mos-sql", "native-table"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("warnings must not mention layer %q; got %v", unwanted, msgs)
		}
	}
}
