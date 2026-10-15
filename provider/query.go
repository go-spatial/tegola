package provider

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/internal/log"
)

// TemporalConstraint selects features whose temporal geometry intersects the
// inclusive interval. Equal bounds select an instant; exactly one nil bound
// selects an open interval. Features without temporal geometry also match.
// Temporal property mappings belong to layer configuration, not the query.
type TemporalConstraint struct {
	Start *time.Time
	End   *time.Time
	// SubNanosecond digits encode the remaining decimal fraction of one
	// nanosecond. Empty or all-zero digits mean no additional fraction.
	StartSubNanosecond string
	EndSubNanosecond   string
	// LeapSecond assigns the endpoint fraction to an inserted UTC second;
	// Time contains the preceding ordinary second and the fractional part.
	StartLeapSecond bool
	EndLeapSecond   bool
}

// Validate rejects empty or reversed temporal constraints without changing them.
func (t TemporalConstraint) Validate() error {
	if t.Start == nil && t.End == nil {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "both bounds are open"}
	}
	if err := validateTemporalEndpoint(t.Start, t.StartSubNanosecond, t.StartLeapSecond); err != nil {
		return err
	}
	if err := validateTemporalEndpoint(t.End, t.EndSubNanosecond, t.EndLeapSecond); err != nil {
		return err
	}
	if t.Start != nil && t.End != nil && compareTemporalEndpoints(*t.Start, t.StartSubNanosecond, t.StartLeapSecond, *t.End, t.EndSubNanosecond, t.EndLeapSecond) > 0 {
		return InvalidFeatureQueryError{Field: "temporal", Reason: "start follows end"}
	}
	return nil
}

// FeatureQuery describes a bounded non-tile query. All dimensions are combined
// with AND; bounds are combined with OR, as are the members of IDs.
// Empty Bounds and Bounds3D impose no spatial restriction; empty IDs or Fields
// impose no identity or property restriction.
// Fields restrict properties only; feature identity and geometry remain present.
// Selected public SQL NULL properties remain present with nil values. Private
// and unselected properties are absent.
// Providers must neither mutate inputs nor retain them after QueryFeatures returns.
type FeatureQuery struct {
	// Bounds are closed horizontal extents [minX, minY, maxX, maxY] in one CRS.
	// Wrapping bounds must be split into ordinary extents by the caller.
	// Bounds do not represent or imply support for vertical bounding boxes.
	Bounds []geom.Extent
	// Bounds3D are closed XYZ extents; do not combine them with Bounds.
	Bounds3D []Extent3D
	// BoundsVerticalCRS declares the height reference for Bounds3D.
	BoundsVerticalCRS string
	// BoundsSRID must be positive with either bounds slice and zero without both. A numeric
	// SRID alone does not establish provider support for that CRS or transforms.
	BoundsSRID uint64
	// BoundsCRSDefinition pins the complete horizontal query definition for
	// Part 2 callers. Empty preserves the existing numeric CRS query contract.
	// A nonempty definition is authoritative even when BoundsSRID equals the
	// source identifier; providers validate and own its converter before I/O.
	BoundsCRSDefinition string
	Temporal            *TemporalConstraint
	// Filter is combined with the other query dimensions by AND. A nil filter
	// imposes no restriction; nonnil filters require explicit backend support.
	Filter *FilterExpression
	IDs    []uint64
	// Limit must be positive; callers supply defaults and publication limits.
	Limit uint
	// Offset skips distinct matches in stable ascending feature-ID order.
	// Paging does not promise snapshot consistency across concurrent mutations.
	Offset uint64
	Fields []string
}

// Validate checks structural query invariants. Providers additionally validate
// layer mappings, fields, supported CRS and their operational bounds before I/O.
func (q FeatureQuery) Validate() error {
	log.Logger().Debug("validating feature query", "bounds_count", len(q.Bounds), "ids_count", len(q.IDs))
	defer log.Logger().Debug("feature query validation finished")
	if q.Limit == 0 {
		return InvalidFeatureQueryError{Field: "limit", Reason: "must be positive"}
	}
	if q.Offset > math.MaxUint64-uint64(q.Limit) {
		return InvalidFeatureQueryError{Field: "offset", Reason: "offset and limit overflow"}
	}
	if len(q.Bounds) != 0 && len(q.Bounds3D) != 0 {
		return InvalidFeatureQueryError{Field: "bounds", Reason: "horizontal and three-dimensional bounds are mutually exclusive"}
	}
	hasBounds := len(q.Bounds) != 0 || len(q.Bounds3D) != 0
	if q.BoundsCRSDefinition != "" && (!hasBounds || strings.TrimSpace(q.BoundsCRSDefinition) == "" || len(q.BoundsCRSDefinition) > MaxFeatureCRSDefinitionBytes || !utf8.ValidString(q.BoundsCRSDefinition) || strings.IndexByte(q.BoundsCRSDefinition, 0) >= 0) {
		return InvalidFeatureQueryError{Field: "bounds_crs_definition", Reason: "requires bounds and a bounded nonblank definition"}
	}
	if !hasBounds && q.BoundsSRID != 0 {
		return InvalidFeatureQueryError{Field: "bounds_srid", Reason: "requires bounds"}
	}
	if hasBounds && q.BoundsSRID == 0 {
		return InvalidFeatureQueryError{Field: "bounds_srid", Reason: "must be positive with bounds"}
	}
	if len(q.Bounds3D) == 0 && q.BoundsVerticalCRS != "" {
		return InvalidFeatureQueryError{Field: "bounds_vertical_crs", Reason: "requires three-dimensional bounds"}
	}
	if len(q.Bounds3D) != 0 && strings.TrimSpace(q.BoundsVerticalCRS) == "" {
		return InvalidFeatureQueryError{Field: "bounds_vertical_crs", Reason: "required with three-dimensional bounds"}
	}
	for _, bounds := range q.Bounds3D {
		for _, coordinate := range bounds {
			if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
				return InvalidFeatureQueryError{Field: "bounds3d", Reason: "coordinates must be finite"}
			}
		}
		for axis := range 3 {
			if bounds[axis] > bounds[axis+3] {
				return InvalidFeatureQueryError{Field: "bounds3d", Reason: "minimum exceeds maximum"}
			}
		}
	}
	for _, bounds := range q.Bounds {
		for _, coordinate := range bounds {
			if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
				return InvalidFeatureQueryError{Field: "bounds", Reason: "coordinates must be finite"}
			}
		}
		if bounds[0] > bounds[2] || bounds[1] > bounds[3] {
			return InvalidFeatureQueryError{Field: "bounds", Reason: "minimum exceeds maximum"}
		}
	}
	if q.Temporal != nil {
		if err := q.Temporal.Validate(); err != nil {
			return err
		}
	}
	if q.Filter != nil {
		if err := q.Filter.Validate(); err != nil {
			return err
		}
	}
	fields := make(map[string]struct{}, len(q.Fields))
	for _, field := range q.Fields {
		if strings.TrimSpace(field) == "" {
			return InvalidFeatureQueryError{Field: "fields", Reason: "contains an empty property name"}
		}
		if _, exists := fields[field]; exists {
			return InvalidFeatureQueryError{Field: "fields", Reason: "contains a duplicate property name"}
		}
		fields[field] = struct{}{}
	}
	return nil
}

// FeatureQueryResult reports distinct matches after spatial-union deduplication.
// NumberReturned counts successfully delivered callbacks. NumberMatched is the
// exact total before paging when known: nil means unknown, a pointer to zero
// means an exact empty result. HasMore requires evidence of another match after
// the page; lookahead must not deliver an extra callback. On error, counts may
// describe partial work and must not be treated as a successful complete result.
type FeatureQueryResult struct {
	NumberReturned uint64
	NumberMatched  *uint64
	HasMore        bool
}

// FeatureQuerier is an optional raw-feature capability beside Tiler. It must not
// implement queries by fabricating a Tile or materializing an entire collection.
// Implementations validate query, layer and a nonnil callback before I/O, then
// deduplicate matches before offset, limit and counting. Callbacks run serially
// and synchronously. Callback errors stop scanning immediately and remain in the
// returned error chain. Check context before I/O and each delivery, preserving
// context.Canceled and context.DeadlineExceeded for errors.Is. Do not substitute
// the older formatted ErrCanceled. No callbacks occur after return or failure.
// Do not mutate shared feature geometry or retain query-owned slices/pointers.
// Unsupported query operations return an error matching ErrUnsupported.
type FeatureQuerier interface {
	QueryFeatures(
		ctx context.Context,
		layer string,
		query FeatureQuery,
		fn func(*Feature) error,
	) (FeatureQueryResult, error)
}

// InvalidFeatureQueryError describes a structurally invalid query or invalid
// layer-specific query mapping. Reason must not contain sensitive feature values.
type InvalidFeatureQueryError struct {
	Field  string
	Reason string
}

func (e InvalidFeatureQueryError) Error() string {
	return fmt.Sprintf("provider: invalid feature query %s: %s", e.Field, e.Reason)
}

// FeatureLayerNotFoundError identifies a missing provider layer.
type FeatureLayerNotFoundError struct {
	Layer string
}

func (e FeatureLayerNotFoundError) Error() string {
	return fmt.Sprintf("provider: feature layer %q not found", e.Layer)
}

// ErrFeatureQueryUnsupported identifies unavailable feature-query operations.
// It also matches ErrUnsupported through errors.Is.
var ErrFeatureQueryUnsupported = fmt.Errorf("feature query: %w", ErrUnsupported)

// FeatureQuerier returns the optional capability of a standard provider.
// MVT-only providers are excluded even if they incidentally implement the API.
func (tu TilerUnion) FeatureQuerier() (FeatureQuerier, error) {
	log.Logger().Debug("checking standard provider feature-query capability")
	if tu.Std != nil {
		if querier, ok := tu.Std.(FeatureQuerier); ok {
			return querier, nil
		}
	}
	return nil, ErrFeatureQueryUnsupported
}
