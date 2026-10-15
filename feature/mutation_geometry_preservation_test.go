package feature

import (
	"github.com/alexeydott/tegola/provider"
	"testing"
)

func TestPreservedReplaceGeometryValidation(t *testing.T) {
	schema := &SchemaDescriptor{Geometry: GeometryDescriptor{Nullable: false}}
	valid := provider.Mutation{Op: provider.MutationReplace, FeatureID: 1, GeometryUnchanged: true, IfRevision: "0.1"}
	if err := validateMutationInput(schema, valid); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*provider.Mutation){
		func(m *provider.Mutation) { m.IfRevision = "" },
		func(m *provider.Mutation) { m.Op = provider.MutationInsert },
		func(m *provider.Mutation) { m.Op = provider.MutationUpdate },
		func(m *provider.Mutation) { m.Op = provider.MutationDelete },
		func(m *provider.Mutation) { m.GeometryAbsent = true },
		func(m *provider.Mutation) { m.GeometryWKB = []byte{1} },
		func(m *provider.Mutation) { m.GeometryUnchanged = false },
	} {
		m := valid
		mutate(&m)
		if err := validateMutationInput(schema, m); err == nil {
			t.Fatalf("invalid preservation admitted: %+v", m)
		}
	}
}
