package gpkg

import (
	"testing"

	"github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestIntegrationMOSBoundsQualifier(t *testing.T) {
	layer := &Layer{geometryFormat: codec.FormatMOS, bboxTable: "source.items", bboxFields: codec.BBoxFields{"minx", "maxx", "miny", "maxy"}, mosConfig: codec.MOSConfig{Precision: 1, UnitFactor: 1}}
	got, err := buildBBoxPredicate(layer, geom.NewExtent([2]float64{10, 20}, [2]float64{30, 40}))
	if err != nil {
		t.Fatal(err)
	}
	want := "`source`.`items`.`maxx` >= 100 AND `source`.`items`.`minx` <= 300 AND `source`.`items`.`maxy` >= 200 AND `source`.`items`.`miny` <= 400"
	if got != want {
		t.Fatalf("qualified bounds: %q", got)
	}
	if layer.bboxFields != (codec.BBoxFields{"minx", "maxx", "miny", "maxy"}) {
		t.Fatal("result labels mutated")
	}
}
