package provider

import "context"

// WriteDescriptor describes what a layer admits for writing. It is
// produced by write admission (ADR-0013), never implied by read access.
type WriteDescriptor struct {
	// CreateUnsupportedReason is nonempty when generated feature IDs are unavailable.
	// Other mutation operations may still be admitted.
	CreateUnsupportedReason string
	// Layer is the provider layer name.
	Layer string
	// Table is the physical base relation name.
	Table string
	// IDColumn is the storage primary-key column.
	IDColumn string
	// GeometryColumn is the storage geometry column.
	GeometryColumn string
	// GeometryType is the admitted simple-feature type.
	GeometryType string
	// GeometrySRID is the storage SRID.
	GeometrySRID uint64
	// WritableColumns maps public property names to storage columns.
	WritableColumns map[string]string
	// ReadOnlyColumns lists public properties that cannot be written.
	ReadOnlyColumns []string
	// RevisionColumn names the explicit revision column, empty when the
	// profile does not prove revision-based concurrency.
	RevisionColumn string
	// Domain is the transaction domain identifier.
	Domain string
}

// SchemaProvider is implemented by providers that can describe their
// layers as neutral schemas for validation, XSD/JSON-Schema generation
// and UI forms.
type SchemaProvider interface {
	DescribeSchema(ctx context.Context, layer string) (SchemaDescriptor, error)
}

// SchemaDescriptor is the provider-level view of a layer schema. The
// neutral feature.SchemaDescriptor is built from it.
type SchemaDescriptor struct {
	Layer    string
	Table    string
	IDColumn string
	Columns  []ColumnDescriptor
	Geometry GeometryColumnDescriptor
	Revision string
}

// ColumnDescriptor describes one storage column.
type ColumnDescriptor struct {
	Name     string
	Type     string
	Nullable bool
	// IsDefault reports a server-side default.
	IsDefault bool
	// IsGenerated reports a generated/stored column.
	IsGenerated bool
}

// GeometryColumnDescriptor describes the storage geometry column.
type GeometryColumnDescriptor struct {
	// Nullable reports whether explicit NULL geometry is permitted by storage.
	Nullable  bool
	Name      string
	Type      string
	SRID      uint64
	Dimension string
}
