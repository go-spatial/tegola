package server

import (
	"fmt"

	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

// buildInsertMutation converts a parsed GeoJSON feature into a neutral
// Insert command. Geometry arrives as WKB in the storage CRS (GeoJSON is
// defined as CRS84; the writer validates/transforms per its profile).
func buildInsertMutation(schema *feature.SchemaDescriptor, collection string, gf *geoJSONFeature) (provider.Mutation, error) {
	props, err := mutationInputToProvider(schema, gf.Properties)
	if err != nil {
		return provider.Mutation{}, err
	}
	m := provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: collection,
		Properties: props,
	}
	if gf.HasGeometry {
		wkbBytes, err := geometryToWKB(gf.Geometry)
		if err != nil {
			return provider.Mutation{}, err
		}
		m.GeometryWKB = wkbBytes
		m.GeometrySRID = 4326 // GeoJSON default CRS
	} else if gf.GeometryNull {
		m.GeometryAbsent = true
	}
	return m, nil
}

// buildReplaceMutation converts a parsed GeoJSON feature into Replace.
// A05: complete replacement semantics. Every writable property in the
// schema is set; properties absent from the input become explicit NULL
// (if nullable), use the column default (if HasDefault), or cause a
// validation error (if required). System fields (PK) are never touched.
func buildReplaceMutation(schema *feature.SchemaDescriptor, collection string, featureID uint64, gf *geoJSONFeature) (provider.Mutation, error) {
	if !gf.HasGeometry && !gf.GeometryNull {
		return provider.Mutation{}, fmt.Errorf("replace requires geometry (object or null)")
	}
	m, err := buildInsertMutation(schema, collection, gf)
	if err != nil {
		return provider.Mutation{}, err
	}
	// A05: fill in absent writable properties for complete replacement.
	for _, prop := range schema.Properties {
		name := prop.Name
		if prop.ReadOnly {
			continue // system/read-only fields not touched
		}
		if _, ok := m.Properties[name]; ok {
			continue // already provided
		}
		// Absent: NULL if nullable, default if HasDefault, else error if required.
		if prop.Nullable {
			m.Properties[name] = provider.MutationValue{Null: true}
		} else if prop.HasDefault {
			// The provider resets omitted defaulted columns during Replace.
			continue
		} else if prop.Required {
			return provider.Mutation{}, &provider.MutationError{
				Kind:   provider.MutationErrSchemaViolation,
				Reason: fmt.Sprintf("replace: required property %q is missing", name),
			}
		}
		// The provider also validates omitted columns against native metadata.
	}
	m.Op = provider.MutationReplace
	m.FeatureID = featureID
	return m, nil
}

// buildPatchMutation applies an RFC 7396 merge patch to the current
// feature representation and builds an Update from the diff. null in the
// patch removes the JSON member; the schema decides whether that maps to
// SQL NULL (nullable) or is a validation error (required).
func buildPatchMutation(schema *feature.SchemaDescriptor, collection string, featureID uint64, current features.Feature, patch map[string]interface{}) (provider.Mutation, error) {
	if patch == nil {
		return provider.Mutation{}, fmt.Errorf("patch must be an object")
	}
	merged := applyMergePatch(patchFeatureDocument(current), patch)
	return buildMutationFromMerged(schema, collection, featureID, current, merged)
}

func patchFeatureDocument(current features.Feature) map[string]interface{} {
	doc := map[string]interface{}{
		"type":       "Feature",
		"id":         current.ID,
		"properties": copyProps(current.Properties),
		"geometry":   nil,
	}
	if len(current.Geometry) > 0 && string(current.Geometry) != "null" {
		doc["geometry"] = jsonRawToMap(current.Geometry)
	}
	return doc
}

// buildJSONPatchMutation applies an RFC 6902 JSON Patch to the current
// feature representation and builds an Update from the diff.
// A21: full JSON Patch support with atomic application.
func buildJSONPatchMutation(schema *feature.SchemaDescriptor, collection string, featureID uint64, current features.Feature, ops []JSONPatchOp) (provider.Mutation, error) {
	currentDoc := patchFeatureDocument(current)
	merged, err := applyJSONPatch(currentDoc, ops)
	if err != nil {
		return provider.Mutation{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("JSON Patch failed: %v", err)}
	}
	// Reuse the diff logic from merge patch by converting merged back.
	// For simplicity, build the mutation from merged directly.
	return buildMutationFromMerged(schema, collection, featureID, current, merged)
}

// buildMutationFromMerged builds an Update mutation from a merged document.
func buildMutationFromMerged(schema *feature.SchemaDescriptor, collection string, featureID uint64, current features.Feature, merged map[string]interface{}) (provider.Mutation, error) {
	if merged["type"] != "Feature" || !jsonEqual(merged["id"], current.ID) {
		return provider.Mutation{}, fmt.Errorf("feature type and id are immutable")
	}
	for name := range merged {
		if name != "type" && name != "id" && name != "geometry" && name != "properties" {
			return provider.Mutation{}, fmt.Errorf("unsupported feature member %q", name)
		}
	}
	mergedProps := map[string]interface{}{}
	if value := merged["properties"]; value != nil {
		var ok bool
		mergedProps, ok = value.(map[string]interface{})
		if !ok {
			return provider.Mutation{}, fmt.Errorf("properties must be an object or null")
		}
	}
	changed := map[string]interface{}{}
	for k, v := range mergedProps {
		cv, ok := current.Properties[k]
		if !ok || !jsonEqual(cv, v) {
			changed[k] = v
		}
	}
	for k := range current.Properties {
		if _, ok := mergedProps[k]; !ok {
			changed[k] = nil
		}
	}
	props, err := mutationInputToProvider(schema, changed)
	if err != nil {
		return provider.Mutation{}, err
	}
	m := provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: collection,
		FeatureID:  featureID,
		Properties: props,
	}
	mg := merged["geometry"]
	if mg == nil {
		if len(current.Geometry) > 0 && string(current.Geometry) != "null" {
			m.GeometryAbsent = true
		}
	} else {
		gm, ok := mg.(map[string]interface{})
		if !ok {
			return provider.Mutation{}, fmt.Errorf("geometry must be an object or null")
		}
		g, err := parseGeoJSONGeometry(gm)
		if err != nil {
			return provider.Mutation{}, err
		}
		wkbBytes, err := geometryToWKB(g)
		if err != nil {
			return provider.Mutation{}, err
		}
		if !geometryEqual(current.Geometry, wkbBytes) {
			m.GeometryWKB = wkbBytes
			m.GeometrySRID = 4326
		}
	}
	return m, nil
}

func copyProps(in map[string]any) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
