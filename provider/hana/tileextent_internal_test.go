package hana

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

// TestReplaceTokensNilExtentError pins audit P6-20: a tile reporting a
// nil extent must surface as an error from replaceTokens, not as an
// unchecked nil flowing into the query builder. Pre-fix: this panicked
// with a nil-pointer dereference on extent.MaxX().
func TestReplaceTokensNilExtentError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("replaceTokens panicked on a nil-extent tile (audit P6-20): %v", r)
		}
	}()

	l := &Layer{name: "lyr", geomField: "geom", idField: "gid"}
	sql, err := replaceTokens(2, "SELECT 1", l, nil, 3857, fixture.Tile{Z: 14, X: 8, Y: 5, SRID: 3857}, false)
	if err == nil {
		t.Fatalf("replaceTokens must propagate the nil-extent error (audit P6-20), got SQL %q", sql)
	}
}

// TestGetTileExtentRejectsBadExtents pins audit P6-20: getTileExtent
// reports an error for nil and non-finite extents instead of returning
// them unchecked to callers.
func TestGetTileExtentRejectsBadExtents(t *testing.T) {
	nilTile := fixture.Tile{Z: 14, X: 8, Y: 5, SRID: 3857}
	if _, _, err := getTileExtent(nilTile, false); err == nil {
		t.Error("getTileExtent with a nil extent must return an error (audit P6-20)")
	}
	if _, _, err := getTileExtent(nilTile, true); err == nil {
		t.Error("buffered getTileExtent with a nil extent must return an error (audit P6-20)")
	}

	nanBounds := geom.NewExtent([2]float64{math.NaN(), 0}, [2]float64{1, 1})
	nanTile := fixture.Tile{
		Z: 14, X: 8, Y: 5, SRID: 3857,
		Bounds: nanBounds, BufferedBounds: nanBounds,
	}
	if _, _, err := getTileExtent(nanTile, false); err == nil {
		t.Error("getTileExtent with a non-finite extent must return an error (audit P6-20)")
	}
	infBounds := geom.NewExtent([2]float64{0, 0}, [2]float64{math.Inf(1), 1})
	infTile := fixture.Tile{
		Z: 14, X: 8, Y: 5, SRID: 3857,
		Bounds: infBounds, BufferedBounds: infBounds,
	}
	if _, _, err := getTileExtent(infTile, true); err == nil {
		t.Error("buffered getTileExtent with a non-finite extent must return an error (audit P6-20)")
	}
}

// TestGetTileExtentPropagatesSrid: a valid extent passes through with its
// SRID alongside the error-free result.
func TestGetTileExtentPropagatesSrid(t *testing.T) {
	bounds := geom.NewExtent([2]float64{-1, -2}, [2]float64{3, 4})
	goodTile := fixture.Tile{
		Z: 14, X: 8, Y: 5, SRID: 4326,
		Bounds: bounds, BufferedBounds: bounds,
	}
	extent, srid, err := getTileExtent(goodTile, false)
	if err != nil {
		t.Fatalf("getTileExtent: unexpected error: %v", err)
	}
	if srid != 4326 {
		t.Errorf("srid = %v, expected 4326", srid)
	}
	if extent == nil || extent[0] != -1 || extent[3] != 4 {
		t.Errorf("extent = %v, expected [-1 -2 3 4]", extent)
	}
}
