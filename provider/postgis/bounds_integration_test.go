package postgis

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
)

func TestQualifiedTileBoundsPreserveFeatureProfile(t *testing.T) {
	f := featureTestProfile()
	layer := Layer{name: "items", bboxTable: "c", bboxFields: codec.DefaultBBoxFields(),
		mosConfig: mosfixture.Config(), geometryFormat: codec.FormatMOS, srid: tegola.WebMercator,
		feature: f}
	before := *f
	fields := append([]string(nil), layer.bboxFields[:]...)
	query, err := replaceTokens("SELECT id,geom FROM source c WHERE !BBOX!", &layer,
		provider.NewTile(0, 0, 0, 0, tegola.WebMercator), false)
	if err != nil || !strings.Contains(query, `"c"."MINX"`) {
		t.Fatalf("qualified tile predicate: %q, %v", query, err)
	}
	if !reflect.DeepEqual(*f, before) || !reflect.DeepEqual(layer.bboxFields[:], fields) {
		t.Fatal("tile expansion changed feature identity, temporal, CRS or field metadata")
	}
	statement, _ := f.chunkStatement(provider.FeatureQuery{IDs: []uint64{15}}, f.queryColumns(nil), nil)
	if strings.Contains(statement, `"c".`) || !strings.Contains(statement, `l."id"`) {
		t.Fatalf("tile qualifier changed feature physical lineage: %q", statement)
	}
}
