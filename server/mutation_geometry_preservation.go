package server

import (
	"bytes"
	"encoding/json"

	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

// preserveIdenticalReplaceGeometry compares the actual canonical GeoJSON read
// representation, never a tolerance or a transformed approximation. The native
// provider rechecks the pinned revision inside its mutation transaction; a row
// changed since this read cannot inherit the preservation decision.
func preserveIdenticalReplaceGeometry(m *provider.Mutation, current features.Feature, revision string) {
	if m.Op != provider.MutationReplace || m.GeometryWKB == nil || m.GeometryAbsent || revision == "" || m.GeometrySRID != 4326 {
		return
	}
	if m.IfRevision != "" && m.IfRevision != revision {
		return
	}
	var raw map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(current.Geometry))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil || raw == nil {
		return
	}
	geometry, err := parseGeoJSONGeometry(raw)
	if err != nil {
		return
	}
	canonical, err := geometryToWKB(geometry)
	if err != nil || !bytes.Equal(canonical, m.GeometryWKB) {
		return
	}
	m.GeometryWKB = nil
	m.GeometrySRID = 0
	m.GeometryUnchanged = true
	m.IfRevision = revision
}
