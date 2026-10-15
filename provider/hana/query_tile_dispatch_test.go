package hana

import (
	"context"
	"strings"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestTileRawDispatchPreservesParameters(t *testing.T) {
	for _, format := range []string{"wkb", "wkt", "mos"} {
		t.Run(format, func(t *testing.T) {
			state := &featureMock{}
			p := featureMockProvider(t, state)
			l := p.layers["features"]
			l.srid = 4326
			l.geometryFormat = format
			l.mosConfig = codec.MOSConfig{Precision: 7, UnitFactor: 1}
			l.sql = `SELECT "id","geom" FROM "source" WHERE "id"=!VALUE!`
			l.bboxFields = codec.BBoxFields{"minx", "maxx", "miny", "maxy"}
			p.layers["features"] = l
			params := provider.Params{"!VALUE!": {Token: "!VALUE!", SQL: "?", Value: int64(17)}}
			err := p.TileFeatures(context.Background(), "features", provider.NewTile(0, 0, 0, 0, tegola.WebMercator), params, func(*provider.Feature) error { return nil })
			if err != nil || len(state.queries) != 1 {
				t.Fatalf("dispatch: %v queries=%v", err, state.queries)
			}
			if strings.Contains(state.queries[0], "!VALUE!") || !strings.Contains(state.queries[0], `"id"=$1`) {
				t.Fatalf("SQL changed: %s", state.queries[0])
			}
			args := state.args[0]
			if len(args) != 1 || args[0].Value != int64(17) {
				t.Fatalf("raw caller parameters lost: %#v", args)
			}
		})
	}
}
