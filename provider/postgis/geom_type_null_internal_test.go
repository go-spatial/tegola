package postgis

import (
	"github.com/alexeydott/geom"
	"reflect"
	"testing"
)

func TestInspectGeomTypeSkipsNullRows(t *testing.T) {
	for _, column := range []string{"geom", "st_geometrytype"} {
		t.Run(column, func(t *testing.T) {
			layer := &Layer{name: "nullable", geomField: "geom"}
			rows := &probeFakeRows{columns: []string{column}, rows: [][]any{{nil}, {"ST_Point"}, {nil}}}
			if err := inspectGeomTypeRows(layer, "probe", rows); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(layer.geomType, geom.Point{}) {
				t.Fatalf("type = %#v, want Point", layer.geomType)
			}
		})
	}
	layer := &Layer{name: "all_null", geomField: "geom"}
	if err := inspectGeomTypeRows(layer, "probe", &probeFakeRows{columns: []string{"geom"}, rows: [][]any{{nil}}}); err != nil {
		t.Fatal(err)
	}
	if layer.geomType != nil {
		t.Fatalf("NULL-only sample fabricated type %#v", layer.geomType)
	}
	if err := inspectGeomTypeRows(layer, "probe", &probeFakeRows{columns: []string{"geom"}, rows: [][]any{{"ST_Unsupported"}}}); err == nil {
		t.Fatal("unsupported non-NULL geometry type accepted")
	}
}
