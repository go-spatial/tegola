package provider

import (
	"strings"
	"unicode/utf8"
)

const MaxFeatureCRSDefinitionBytes = 65536

// FeatureCRSDefinition is copied registration evidence for source coordinates.
// Definition denotes internal XY order; wire axis order belongs to the service.
// Authority and code are set only by provider provenance, never guessed from a
// numeric identifier or equivalent projection text.
type FeatureCRSDefinition struct {
	HorizontalSRID     uint64
	Definition         string
	CanonicalAuthority string
	CanonicalCode      string
	Spatial            SpatialMetadata
}

// FeatureCRSLayerInfo is an optional Part 2 source proof. ErrUnsupported leaves
// existing Core and tile capabilities available.
type FeatureCRSLayerInfo interface {
	FeatureCRSDefinition() (FeatureCRSDefinition, error)
}

// Validate checks bounded proof shape; provider admission owns its provenance
// and the supported mathematics are separately validated by the converter.
func (d FeatureCRSDefinition) Validate() error {
	if d.HorizontalSRID == 0 || len(d.Definition) == 0 || len(d.Definition) > MaxFeatureCRSDefinitionBytes || strings.TrimSpace(d.Definition) == "" || !utf8.ValidString(d.Definition) || strings.IndexByte(d.Definition, 0) >= 0 {
		return InvalidFeatureQueryError{Field: "source_crs", Reason: "source definition is unavailable or invalid"}
	}
	if len(d.CanonicalAuthority) > 64 || len(d.CanonicalCode) > 64 || !utf8.ValidString(d.CanonicalAuthority) || !utf8.ValidString(d.CanonicalCode) || strings.IndexByte(d.CanonicalAuthority, 0) >= 0 || strings.IndexByte(d.CanonicalCode, 0) >= 0 || ((d.CanonicalAuthority == "") != (d.CanonicalCode == "")) {
		return InvalidFeatureQueryError{Field: "source_crs", Reason: "source authority proof is invalid"}
	}
	if d.CanonicalAuthority != "" && (strings.TrimSpace(d.CanonicalAuthority) != d.CanonicalAuthority || strings.TrimSpace(d.CanonicalCode) != d.CanonicalCode) {
		return InvalidFeatureQueryError{Field: "source_crs", Reason: "source authority proof is invalid"}
	}
	return d.Spatial.Validate()
}
