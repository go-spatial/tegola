package features

import (
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/provider"
	"testing"
)

func TestProviderSchemaWriteMetadata(t *testing.T) {
	schema := providerSchemaToFeature("sites", provider.SchemaDescriptor{
		IDColumn: "id", Revision: "revision", Geometry: provider.GeometryColumnDescriptor{Name: "geom", Dimension: "XYZ", SRID: 4326},
		Columns: []provider.ColumnDescriptor{
			{Name: "id", Type: "INTEGER"}, {Name: "geom", Type: "POINT"},
			{Name: "required", Type: "TEXT"}, {Name: "optional", Type: "TEXT", Nullable: true},
			{Name: "defaulted", Type: "INTEGER", IsDefault: true},
			{Name: "generated", Type: "INTEGER", IsGenerated: true}, {Name: "revision", Type: "INTEGER"},
		},
	})
	if schema.Geometry.Dimension != feature.DimXYZ {
		t.Fatalf("lost dimension: %s", schema.Geometry.Dimension)
	}
	if len(schema.Properties) != 5 {
		t.Fatalf("storage identity/geometry exposed: %+v", schema.Properties)
	}
	for _, name := range []string{"generated", "revision"} {
		p, _ := schema.Property(name)
		if !p.ReadOnly || p.Required {
			t.Errorf("%s must be read-only and not required: %+v", name, p)
		}
	}
	if p, _ := schema.Property("required"); !p.Required {
		t.Fatal("nonnullable column without default must be required")
	}
	for _, name := range []string{"optional", "defaulted"} {
		if p, _ := schema.Property(name); p.Required {
			t.Errorf("%s must be optional", name)
		}
	}
}
