package mysql

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestFeatureLiveCRSDefinition(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, connection := featureLiveDatabase(t, flavor)
			table := featureLiveTable(t, db, "id BIGINT UNSIGNED UNIQUE, geom TEXT")
			for _, row := range [][]any{{1, "POINT(0 0)"}, {2, "POINT(15 30)"}, {3, "POINT(15 30)"}, {4, nil}} {
				if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" (id,geom) VALUES (?,?)", row...); err != nil {
					t.Fatal(err)
				}
			}
			p := featureLiveProvider(t, connection, map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"id"}})
			proof, err := p.layers["items"].FeatureCRSDefinition()
			if err != nil || proof.CanonicalCode != "4326" {
				t.Fatal("source proof", proof, err)
			}
			def, _ := crsconfig.CanonicalFeatureDefinition(3857)
			query := provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, BoundsCRSDefinition: def, Bounds: []geom.Extent{{1669792, 3503549, 1669793, 3503550}}}
			run := func(q provider.FeatureQuery) ([]uint64, provider.FeatureQueryResult, error) {
				var ids []uint64
				result, err := p.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				return ids, result, err
			}
			t.Run("same_numeric_different_definition", func(t *testing.T) {
				ids, result, err := run(query)
				if err != nil || !reflect.DeepEqual(ids, []uint64{2, 3, 4}) || result.NumberMatched == nil || *result.NumberMatched != 3 {
					t.Fatal(ids, result, err)
				}
			})
			t.Run("exact_before_offset", func(t *testing.T) {
				q := query
				q.Limit, q.Offset = 1, 1
				ids, result, err := run(q)
				if err != nil || !reflect.DeepEqual(ids, []uint64{3}) || !result.HasMore {
					t.Fatal(ids, result, err)
				}
			})
			t.Run("numeric_core_unchanged", func(t *testing.T) {
				q := query
				q.BoundsCRSDefinition = ""
				ids, _, err := run(q)
				if err != nil || !reflect.DeepEqual(ids, []uint64{4}) {
					t.Fatal(ids, err)
				}
			})
			t.Run("frozen_query_ignores_registry_override", func(t *testing.T) {
				previous, _ := basic.EffectiveProj4Definition(3857)
				alternate, _ := crsconfig.CanonicalFeatureDefinition(32633)
				if err := basic.RegisterProj4SRID(3857, alternate); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := basic.RegisterProj4SRID(3857, previous); err != nil {
						t.Error(err)
					}
				}()
				ids, _, err := run(query)
				if err != nil || !reflect.DeepEqual(ids, []uint64{2, 3, 4}) {
					t.Fatal(ids, err)
				}
			})
			t.Run("unsupported_definition_zero_callbacks", func(t *testing.T) {
				q := query
				q.BoundsCRSDefinition = "+proj=longlat +datum=NAD83"
				ids, _, err := run(q)
				if !errors.Is(err, provider.ErrUnsupported) || len(ids) != 0 {
					t.Fatal(ids, err)
				}
			})
			t.Run("frozen_source_ignores_registry_override", func(t *testing.T) {
				previous, _ := basic.EffectiveProj4Definition(3857)
				canonical, _ := crsconfig.CanonicalFeatureDefinition(3857)
				if err := basic.RegisterProj4SRID(3857, canonical); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := basic.RegisterProj4SRID(3857, previous); err != nil {
						t.Error(err)
					}
				}()
				sourceTable := featureLiveTable(t, db, "id BIGINT UNSIGNED UNIQUE, geom TEXT")
				if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(sourceTable)+" (id,geom) VALUES (?,?)", 1, "POINT(1669792.3618991035 3503549.843504374)"); err != nil {
					t.Fatal(err)
				}
				sourceProvider := featureLiveProvider(t, connection, map[string]any{"name": "items", "tablename": sourceTable, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 3857, "fields": []string{"id"}})
				alternate, _ := crsconfig.CanonicalFeatureDefinition(32633)
				if err := basic.RegisterProj4SRID(3857, alternate); err != nil {
					t.Fatal(err)
				}
				target, _ := crsconfig.CanonicalFeatureDefinition(4326)
				q := provider.FeatureQuery{Limit: 10, BoundsSRID: 3857, BoundsCRSDefinition: target, Bounds: []geom.Extent{{14, 29, 16, 31}}}
				var ids []uint64
				_, err := sourceProvider.QueryFeatures(context.Background(), "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{1}) {
					t.Fatal("frozen source changed", ids, err)
				}
			})
			t.Run("cancellation_chain", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				called := false
				_, err := p.QueryFeatures(ctx, "items", query, func(*provider.Feature) error { called = true; return nil })
				if !errors.Is(err, context.Canceled) || called {
					t.Fatal(called, err)
				}
			})
		})
	}
}
