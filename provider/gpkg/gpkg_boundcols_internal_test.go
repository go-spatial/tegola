//go:build cgo

package gpkg

import (
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// TestMatchBoundColumns covers audit N6: the explicitly configured
// bbox_*_fieldname columns win over the legacy-name autodetection when the
// table carries all four configured names (actual case kept for SQL
// quoting, query order [minx, maxx, miny, maxy]); the legacy names are
// consulted only when the configured set is not fully present.
func TestMatchBoundColumns(t *testing.T) {
	t.Run("configured columns win over legacy names", func(t *testing.T) {
		cols := []string{"id", "geom", "x0", "x1", "y0", "y1", "minx", "maxx", "miny", "maxy"}
		got := matchBoundColumns(cols, codec.BBoxFields{"x0", "x1", "y0", "y1"})
		if got == nil {
			t.Fatal("matchBoundColumns returned nil for configured columns present in the table")
		}
		want := [4]string{"x0", "x1", "y0", "y1"}
		if *got != want {
			t.Errorf("bound columns = %v, want %v", *got, want)
		}
	})

	t.Run("configured names match case-insensitively keeping actual case", func(t *testing.T) {
		cols := []string{"X0", "x1", "Y0", "y1"}
		got := matchBoundColumns(cols, codec.BBoxFields{"x0", "X1", "y0", "Y1"})
		if got == nil {
			t.Fatal("matchBoundColumns returned nil for case-insensitive matches")
		}
		want := [4]string{"X0", "x1", "Y0", "y1"}
		if *got != want {
			t.Errorf("bound columns = %v, want %v (actual case preserved for quoting)", *got, want)
		}
	})

	t.Run("missing configured column falls back", func(t *testing.T) {
		cols := []string{"id", "geom", "x0", "x1", "y1"}
		if got := matchBoundColumns(cols, codec.BBoxFields{"x0", "x1", "y0", "y1"}); got != nil {
			t.Errorf("matchBoundColumns = %v, want nil when one configured column is missing", *got)
		}
	})

	t.Run("default names behave like the legacy autodetect", func(t *testing.T) {
		cols := []string{"id", "geom", "minx", "maxx", "miny", "maxy"}
		got := matchBoundColumns(cols, codec.DefaultBBoxFields())
		if got == nil {
			t.Fatal("matchBoundColumns returned nil for the default names")
		}
		want := [4]string{"minx", "maxx", "miny", "maxy"}
		if *got != want {
			t.Errorf("bound columns = %v, want %v", *got, want)
		}
	})

	t.Run("detectBoundColumns legacy regression", func(t *testing.T) {
		if got := detectBoundColumns([]string{"id", "geom", "x0", "x1", "y0", "y1"}); got != nil {
			t.Errorf("detectBoundColumns = %v, want nil for non-standard names", *got)
		}
		got := detectBoundColumns([]string{"Minx", "MAXX", "miny", "maxy"})
		if got == nil {
			t.Fatal("detectBoundColumns returned nil for legacy names")
		}
		want := [4]string{"Minx", "MAXX", "miny", "maxy"}
		if *got != want {
			t.Errorf("detectBoundColumns = %v, want %v", *got, want)
		}
	})
}

// TestRawBoundsSQLUsesConfiguredColumns covers audit N6 at the SQL layer:
// the raw-table bounds filter must reference the configured non-standard
// column names (quoted verbatim), so the tile query filters in SQL instead
// of scanning the whole table.
func TestRawBoundsSQLUsesConfiguredColumns(t *testing.T) {
	cols := []string{"id", "geom", "x0", "x1", "y0", "y1", "minx", "maxx", "miny", "maxy"}
	matched := matchBoundColumns(cols, codec.BBoxFields{"x0", "x1", "y0", "y1"})
	if matched == nil {
		t.Fatal("matchBoundColumns returned nil for configured columns present in the table")
	}
	l := &Layer{
		geometryFormat:  codec.FormatWKB,
		boundFieldnames: matched,
	}
	extent := geom.NewExtent([2]float64{10, 20}, [2]float64{30, 40})
	pred := rawBoundsSQL(l, extent)
	if pred == "" {
		t.Fatal("rawBoundsSQL returned an empty predicate for a table with configured bounds columns")
	}
	for _, col := range []string{"`x0`", "`x1`", "`y0`", "`y1`"} {
		if !strings.Contains(pred, col) {
			t.Errorf("predicate must filter over the configured column %s: %q", col, pred)
		}
	}
	// the legacy names are plain user columns here: the filter must not
	// reference them
	for _, col := range []string{"`minx`", "`maxx`", "`miny`", "`maxy`"} {
		if strings.Contains(pred, col) {
			t.Errorf("predicate must not reference the non-bounds column %s: %q", col, pred)
		}
	}
}
