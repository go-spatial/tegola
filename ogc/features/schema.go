package features

import (
	"strings"

	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/provider"
)

// providerSchemaToFeature converts a provider schema descriptor into the
// neutral feature.SchemaDescriptor. Public property names are the storage
// column names for tablename layers; the ID and geometry columns are not
// public properties.
func providerSchemaToFeature(collectionID string, psd provider.SchemaDescriptor) *feature.SchemaDescriptor {
	sd := &feature.SchemaDescriptor{
		Collection:    collectionID,
		IDColumn:      psd.IDColumn,
		SchemaVersion: "1",
		Geometry: feature.GeometryDescriptor{
			Name:      "geometry",
			Nullable:  psd.Geometry.Nullable,
			Column:    psd.Geometry.Name,
			Type:      strings.ToLower(psd.Geometry.Type),
			Dimension: feature.DimXY,
			SRID:      psd.Geometry.SRID,
		},
	}
	if psd.Revision != "" {
		sd.Revision = feature.RevisionStrategy{Column: psd.Revision}
	}
	if psd.Geometry.Dimension != "" {
		sd.Geometry.Dimension = feature.GeometryDimension(strings.ToUpper(psd.Geometry.Dimension))
	}
	for _, c := range psd.Columns {
		if c.Name == psd.IDColumn || c.Name == psd.Geometry.Name {
			continue
		}
		sd.Properties = append(sd.Properties, feature.PropertyDescriptor{
			Name:       c.Name,
			Column:     c.Name,
			Type:       sqliteTypeToLogical(c.Type),
			Nullable:   c.Nullable,
			HasDefault: c.IsDefault,
			ReadOnly:   c.IsGenerated || c.Name == psd.Revision,
			Required:   !c.Nullable && !c.IsDefault && !c.IsGenerated && c.Name != psd.Revision,
		})
	}
	return sd
}

// sqliteTypeToLogical maps a storage column type to a logical type.
// Unknown types default to string (conservative: text round-trips).
func sqliteTypeToLogical(decl string) feature.LogicalType {
	u := strings.ToUpper(strings.TrimSpace(decl))
	switch {
	case strings.Contains(u, "INT"):
		return feature.TypeInteger
	case strings.Contains(u, "REAL"), strings.Contains(u, "FLOAT"), strings.Contains(u, "DOUBLE"):
		return feature.TypeDecimal
	case strings.Contains(u, "NUMERIC"), strings.Contains(u, "DECIMAL"):
		return feature.TypeDecimal
	case strings.Contains(u, "BOOL"):
		return feature.TypeBoolean
	case strings.Contains(u, "DATE"), strings.Contains(u, "TIME"):
		return feature.TypeDateTime
	default:
		return feature.TypeString
	}
}
