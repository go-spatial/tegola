package server

import (
	"encoding/json"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

func TestReplaceGeometryPreservationRequiresExactRevisionBoundIdentity(t *testing.T) {
	for _, tt := range []struct {
		name                string
		point               geom.Point
		op                  provider.MutationOp
		revision, condition string
		srid                uint64
		want                bool
	}{
		{"identical pinned", geom.Point{1, 2}, provider.MutationReplace, "0.3", "0.3", 4326, true},
		{"identical unconditional pins read", geom.Point{1, 2}, provider.MutationReplace, "0.3", "", 4326, true},
		{"tiny change remains geometry write", geom.Point{1.000000000001, 2}, provider.MutationReplace, "0.3", "", 4326, false},
		{"no revision cannot preserve", geom.Point{1, 2}, provider.MutationReplace, "", "", 4326, false},
		{"different revision cannot preserve", geom.Point{1, 2}, provider.MutationReplace, "0.4", "0.3", 4326, false},
		{"patch remains independent", geom.Point{1, 2}, provider.MutationUpdate, "0.3", "", 4326, false},
		{"other input CRS", geom.Point{1, 2}, provider.MutationReplace, "0.3", "", 3857, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := geometryToWKB(tt.point)
			if err != nil {
				t.Fatal(err)
			}
			m := provider.Mutation{Op: tt.op, GeometryWKB: payload, GeometrySRID: tt.srid, IfRevision: tt.condition}
			preserveIdenticalReplaceGeometry(&m, features.Feature{Geometry: json.RawMessage(`{"type":"Point","coordinates":[1,2]}`)}, tt.revision)
			if m.GeometryUnchanged != tt.want {
				t.Fatalf("preserved=%v want%v", m.GeometryUnchanged, tt.want)
			}
			if tt.want && (m.GeometryWKB != nil || m.IfRevision != tt.revision) {
				t.Fatal("preservation lost CAS or retained rewriting payload")
			}
			if !tt.want && m.GeometryWKB == nil {
				t.Fatal("changed geometry discarded")
			}
		})
	}
}
