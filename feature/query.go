package feature

import (
	"context"

	"github.com/alexeydott/tegola/provider"
)

// FeatureRecord is the neutral owned representation of one feature,
// produced before any wire-format serialization (GeoJSON, GML).
// GML must not be produced through GeoJSON. See ADR-0010.
type FeatureRecord struct {
	// Collection is the public collection name.
	Collection string
	// ID is the public numeric feature ID.
	ID uint64
	// Key is the physical identity used by locks, revisions and audit.
	Key PhysicalFeatureKey
	// GeometryWKB carries the original storage geometry (not an MVT
	// clipped/simplified fragment). Nil means absent geometry.
	GeometryWKB []byte
	// GeometrySRID is the storage SRID of GeometryWKB.
	GeometrySRID uint64
	// Properties holds typed public property values.
	Properties map[string]TypedValue
	// Revision is the backend revision token, empty when the profile
	// does not prove one.
	Revision string
	// SchemaVersion pins the descriptor this record was read with.
	SchemaVersion string
}

// QueryService executes neutral read queries against providers and
// returns owned FeatureRecords. The existing GeoJSON facade stays on
// top of it; behavior of the read API is unchanged (W06).
type QueryService struct {
	// QuerierFor resolves the provider.FeatureQuerier for a collection.
	// A02: also returns the physical domain ID (e.g. "mysql:<hash>") to pin
	// the binding; the Domain in PhysicalFeatureKey must identify the physical
	// DB, not just the layer name.
	QuerierFor func(collection string) (provider.FeatureQuerier, string, string, error)
	// SchemaFor resolves the schema descriptor for a collection.
	SchemaFor func(collection string) (*SchemaDescriptor, error)
}

// RawQuery runs query against the collection's provider and converts
// each provider.Feature into a FeatureRecord with typed properties.
// Fields restrict properties only; identity and geometry stay present.
func (s *QueryService) RawQuery(ctx context.Context, collection string, query provider.FeatureQuery) ([]*FeatureRecord, provider.FeatureQueryResult, error) {
	querier, layer, domain, err := s.QuerierFor(collection)
	if err != nil {
		return nil, provider.FeatureQueryResult{}, err
	}
	schema, err := s.SchemaFor(collection)
	if err != nil {
		return nil, provider.FeatureQueryResult{}, err
	}
	var out []*FeatureRecord
	res, err := querier.QueryFeatures(ctx, layer, query, func(f *provider.Feature) error {
		rec, convErr := s.convert(collection, layer, domain, schema, f)
		if convErr != nil {
			return convErr
		}
		out = append(out, rec)
		return nil
	})
	if err != nil {
		return nil, provider.FeatureQueryResult{}, err
	}
	return out, res, nil
}

func (s *QueryService) convert(collection, layer, domain string, schema *SchemaDescriptor, f *provider.Feature) (*FeatureRecord, error) {
	rec := &FeatureRecord{
		Collection:    collection,
		ID:            f.ID,
		GeometrySRID:  f.SRID,
		Properties:    make(map[string]TypedValue, len(f.Tags)),
		SchemaVersion: schema.SchemaVersion,
	}
	// A02: Domain identifies the physical DB, pinned at query time.
	rec.Key = PhysicalFeatureKey{
		Domain:   domain,
		Relation: schema.Collection,
		PK:       uint64ToString(f.ID),
	}
	if f.Geometry != nil {
		wkb, err := encodeGeometryWKB(f.Geometry)
		if err != nil {
			return nil, err
		}
		rec.GeometryWKB = wkb
	}
	for name, raw := range f.Tags {
		tv := toTypedValue(raw)
		if desc, ok := schema.Property(name); ok {
			// Keep the schema-declared type; nil stays null-typed.
			if tv.State == ValueNull {
				tv.Type = desc.Type
			}
		}
		rec.Properties[name] = tv
	}
	return rec, nil
}

// toTypedValue converts a provider tag value to a TypedValue without
// passing integers or decimals through float64.
func toTypedValue(raw interface{}) TypedValue {
	if raw == nil {
		return TypedValue{State: ValueNull, Type: TypeString}
	}
	switch v := raw.(type) {
	case int64:
		return TypedValue{Type: TypeInteger, State: ValuePresent, Integer: v}
	case int:
		return TypedValue{Type: TypeInteger, State: ValuePresent, Integer: int64(v)}
	case string:
		if v == "" {
			return TypedValue{Type: TypeString, State: ValueEmpty, String: v}
		}
		return TypedValue{Type: TypeString, State: ValuePresent, String: v}
	case bool:
		return TypedValue{Type: TypeBoolean, State: ValuePresent, Boolean: v}
	case float64:
		// Provider decoded a real column: keep it as decimal text via
		// strconv to avoid further float64 handling downstream.
		return TypedValue{Type: TypeDecimal, State: ValuePresent, Decimal: formatDecimal(v)}
	default:
		return TypedValue{Type: TypeString, State: ValuePresent, String: stringify(raw)}
	}
}
