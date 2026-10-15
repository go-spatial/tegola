package atlas

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/provider"
)

// Exercise the production encode boundary with meter coordinates: a five-unit
// deviation in the output tile must simplify under the default ten-unit budget.
func TestEncodeSimplificationDefaultsAndOptOuts(t *testing.T) {
	enabled := !strings.Contains(strings.ToLower(os.Getenv("TEGOLA_OPTIONS")), "dontsimplifygeo")
	if simplifyGeometries != enabled {
		t.Fatalf("default simplification = %v, want %v", simplifyGeometries, enabled)
	}
	old, oldZoom := simplifyGeometries, simplificationMaxZoom
	defer func() { simplifyGeometries, simplificationMaxZoom = old, oldZoom }()
	simplificationMaxZoom = 10
	for _, tc := range []struct {
		name          string
		zoom          slippy.Zoom
		enabled, dont bool
		want          int
	}{
		{"default", 2, true, false, 2},
		{"layer opt out", 2, true, true, 5},
		{"global opt out", 2, false, false, 5},
		{"zoom limit", 10, true, false, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			simplifyGeometries = tc.enabled
			tile := slippy.Tile{Z: tc.zoom, X: 1, Y: 1}
			ptile := provider.NewTile(tile.Z, tile.X, tile.Y, 0, 3857)
			ext, _ := ptile.Extent()
			unit := (ext.MaxX() - ext.MinX()) / 4096
			geo := geom.LineString{}
			for i, dy := range []float64{0, 5, 0, 5, 0} {
				geo = append(geo, [2]float64{ext.MinX() + float64(100+i*100)*unit, ext.MinY() + (100+dy)*unit})
			}
			layer := mvt.Layer{Name: "test"}
			err := (Map{}).encodeMVTFeature(context.Background(), Layer{DontSimplify: tc.dont, DontClip: true, DontClean: true}, tile, ptile, &layer, &provider.Feature{ID: 1}, geo)
			if err != nil {
				t.Fatal(err)
			}
			fs := layer.Features()
			if len(fs) != 1 {
				t.Fatalf("features = %d", len(fs))
			}
			line, ok := fs[0].Geometry.(geom.LineString)
			if !ok || len(line) != tc.want {
				t.Fatalf("geometry = %#v, want %d vertices", fs[0].Geometry, tc.want)
			}
		})
	}
}
