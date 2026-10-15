package provider_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

// This adapter exercises harness mechanics only. It is not provider/database parity evidence.
func TestFeatureQuerierContractReference(t *testing.T) {
	for _, mode := range []querytest.CountMode{querytest.ExactCount, querytest.UnknownCount, querytest.OptionalCount} {
		t.Run(fmt.Sprintf("count mode %d", mode), func(t *testing.T) {
			querytest.Run(t, func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
				if fixture.InvalidTemporalMapping {
					return querytest.Instance{SetupError: fmt.Errorf("register reference layer: %w", provider.InvalidFeatureQueryError{Field: "temporal", Reason: "invalid mapping"}), CountMode: mode}
				}
				return querytest.Instance{Querier: &contractReference{fixture: fixture, mode: mode}, Layer: "features", CountMode: mode}
			})
		})
	}
}

type contractReference struct {
	fixture querytest.Fixture
	mode    querytest.CountMode
}

func (r *contractReference) QueryFeatures(
	ctx context.Context,
	layer string,
	q provider.FeatureQuery,
	callback func(*provider.Feature) error,
) (provider.FeatureQueryResult, error) {
	var result provider.FeatureQueryResult
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := q.Validate(); err != nil {
		return result, err
	}
	if layer != "features" {
		return result, provider.FeatureLayerNotFoundError{Layer: layer}
	}
	if callback == nil {
		return result, provider.InvalidFeatureQueryError{Field: "callback", Reason: "must be nonnil"}
	}
	if r.fixture.InvalidTemporalMapping && q.Temporal != nil {
		return result, provider.InvalidFeatureQueryError{Field: "temporal", Reason: "invalid mapping"}
	}
	matches := []querytest.Row{}
	for _, row := range r.fixture.Rows {
		if row.MissingID || row.Metadata {
			continue
		}
		if row.MalformedGeometry != "" {
			return result, errors.New("explicit geometry decode failed")
		}
		if len(q.IDs) > 0 {
			found := false
			for _, id := range q.IDs {
				if id == row.Feature.ID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if len(q.Bounds) > 0 && row.Feature.Geometry != nil {
			hit := false
			for _, bounds := range q.Bounds {
				if referenceGeometryIntersects(row.Feature.Geometry, bounds) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		if q.Temporal != nil && (row.Start != nil || row.End != nil) {
			if q.Temporal.Start != nil && row.End != nil && row.End.Before(*q.Temporal.Start) {
				continue
			}
			if q.Temporal.End != nil && row.Start != nil && row.Start.After(*q.Temporal.End) {
				continue
			}
		}
		matches = append(matches, row)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Feature.ID < matches[j].Feature.ID })
	if r.mode == querytest.ExactCount || (r.mode == querytest.OptionalCount && len(q.IDs) == 0) {
		count := uint64(len(matches))
		result.NumberMatched = &count
	}
	for index, row := range matches {
		if uint64(index) < q.Offset {
			continue
		}
		if result.NumberReturned >= uint64(q.Limit) {
			result.HasMore = true
			break
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		feature := referenceCopyFeature(row.Feature)
		if len(q.Fields) > 0 {
			tags := map[string]any{}
			for _, field := range q.Fields {
				if value, ok := feature.Tags[field]; ok {
					tags[field] = value
				}
			}
			feature.Tags = tags
		}
		if err := callback(&feature); err != nil {
			return result, fmt.Errorf("deliver reference feature: %w", err)
		}
		result.NumberReturned++
		if err := ctx.Err(); err != nil {
			return result, err
		}
	}
	return result, nil
}
func referenceGeometryIntersects(geometry geom.Geometry, bounds geom.Extent) bool {
	switch value := geometry.(type) {
	case geom.Point:
		return value[0] >= bounds[0] && value[0] <= bounds[2] && value[1] >= bounds[1] && value[1] <= bounds[3]
	case geom.Collection:
		for _, child := range value {
			if referenceGeometryIntersects(child, bounds) {
				return true
			}
		}
	}
	return false
}
func referenceCopyFeature(f provider.Feature) provider.Feature {
	if f.Tags != nil {
		tags := map[string]any{}
		for key, value := range f.Tags {
			tags[key] = value
		}
		f.Tags = tags
	}

	f.Geometry = referenceCopyGeometry(f.Geometry)
	return f
}
func referenceCopyGeometry(g geom.Geometry) geom.Geometry {
	switch value := g.(type) {
	case geom.Collection:
		out := make(geom.Collection, len(value))
		for i, child := range value {
			out[i] = referenceCopyGeometry(child)
		}
		return out
	case geom.LineString:
		out := make(geom.LineString, len(value))
		copy(out, value)
		return out
	case geom.Polygon:
		out := make(geom.Polygon, len(value))
		for i, ring := range value {
			out[i] = append([][2]float64{}, ring...)
		}
		return out
	default:
		return g
	}
}
