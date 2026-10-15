package mysql

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
)

func TestQualifiedTileBoundsCannotAdmitFeatureSelection(t *testing.T) {
	layer := Layer{name: "items", sql: "SELECT id,geom FROM source c WHERE !BBOX!",
		bboxTable: "c", bboxFields: codec.DefaultBBoxFields(), mosConfig: mosfixture.Config(),
		geometryFormat: codec.FormatMOS, srid: 4326, idFieldname: "id", geomFieldname: "geom"}
	before := layer
	predicate, err := mosBoundsSQL(&layer, &geom.Extent{1, 2, 3, 4})
	if err != nil || !strings.Contains(predicate, "`c`.`MINX`") {
		t.Fatalf("qualified tile predicate: %q, %v", predicate, err)
	}
	// Tile qualification supplies no unique-ID, temporal or spatial lineage.
	// Admission must reject before any database access (the provider has no DB).
	p := &Provider{}
	_, err = p.registerFeatureLayer(layer, dict.Dict{})
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("custom tile SQL admitted without feature_sql: %v", err)
	}
	if !reflect.DeepEqual(layer, before) {
		t.Fatal("bounds predicate or feature admission changed tile metadata")
	}
}
