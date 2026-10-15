package features

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/tegola/ogc/cql2"
	"github.com/alexeydott/tegola/provider"
)

type queryableTestLayer struct {
	*testLayer
	catalog provider.FeatureQueryables
	err     error
	calls   atomic.Int32
}

func (l *queryableTestLayer) FeatureQueryables() (provider.FeatureQueryables, error) {
	l.calls.Add(1)
	return l.catalog, l.err
}
func serviceQueryableLayer(t *testing.T, fields []provider.FeatureQueryable) *queryableTestLayer {
	t.Helper()
	catalog, err := provider.NewFeatureQueryables(fields)
	if err != nil {
		t.Fatal(err)
	}
	return &queryableTestLayer{testLayer: &testLayer{name: "source", srid: 4326}, catalog: catalog}
}
func TestQueryablesSnapshotsTransportSubset(t *testing.T) {
	fields := []provider.FeatureQueryable{{Name: "s", Type: provider.QueryableString, Nullable: true}, {Name: "AND", Type: provider.QueryableString}, {Name: "space name", Type: provider.QueryableInteger}, {Name: "1digit", Type: provider.QueryableInteger}, {Name: "hyphen-name", Type: provider.QueryableInteger}, {Name: "quote\"name", Type: provider.QueryableString}, {Name: "\U000F0000", Type: provider.QueryableString}, {Name: "\u4E2D", Type: provider.QueryableString}}
	layer := serviceQueryableLayer(t, fields)
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(emptyQuerier)}})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := s.Queryables("public")
	if err != nil {
		t.Fatal(err)
	}
	expected := []provider.FeatureQueryable{{Name: "AND", Type: provider.QueryableString}, {Name: "s", Type: provider.QueryableString, Nullable: true}, {Name: "\u4E2D", Type: provider.QueryableString}}
	if !reflect.DeepEqual(catalog.Fields(), expected) {
		t.Fatal(catalog.Fields())
	}
	if len(layer.catalog.Fields()) != len(fields) {
		t.Fatal("provider generic catalog altered")
	}
	layer.err = errors.New("later metadata failure")
	layer.catalog = provider.FeatureQueryables{}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q, err := s.Queryables("public")
			if err != nil {
				t.Error(err)
				return
			}
			detached := q.Fields()
			detached[0].Name = "changed"
			again, _ := s.Queryables("public")
			if !reflect.DeepEqual(again.Fields(), expected) {
				t.Error("snapshot mutated")
			}
		}()
	}
	wg.Wait()
	if layer.calls.Load() != 1 {
		t.Fatal("metadata read after construction")
	}
}
func TestQueryablesOptionalMetadataAndInvalidClaims(t *testing.T) {
	for _, mode := range []string{"missing", "unsupported", "empty", "subset empty", "invalid", "getter error"} {
		t.Run(mode, func(t *testing.T) {
			var layer provider.LayerInfo = &testLayer{name: "source", srid: 4326}
			switch mode {
			case "unsupported":
				l := serviceQueryableLayer(t, nil)
				l.err = fmt.Errorf("no profile: %w", provider.ErrUnsupported)
				layer = l
			case "empty":
				layer = serviceQueryableLayer(t, nil)
			case "subset empty":
				layer = serviceQueryableLayer(t, []provider.FeatureQueryable{{Name: "space name", Type: provider.QueryableString}})
			case "invalid":
				l := serviceQueryableLayer(t, nil)
				l.catalog = provider.FeatureQueryables{}
				layer = l
			case "getter error":
				l := serviceQueryableLayer(t, nil)
				l.err = errors.New("catalog read failed")
				layer = l
			}
			var calls atomic.Int32
			s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(func(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
				calls.Add(1)
				return provider.FeatureQueryResult{}, nil
			})}})
			if mode == "invalid" || mode == "getter error" {
				if err == nil {
					t.Fatal("invalid metadata startup accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			q, e := s.Queryables("public")
			if mode == "missing" || mode == "unsupported" {
				if !errors.Is(e, provider.ErrUnsupported) {
					t.Fatal(e)
				}
			} else if e != nil || len(q.Fields()) != 0 {
				t.Fatal("empty catalog unavailable", e)
			}
			if _, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1}); err != nil || calls.Load() != 1 {
				t.Fatal("Core disabled", err)
			}
			expression, _ := cql2.Parse("TRUE")
			_, err = s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1, Filter: &expression})
			if mode == "missing" || mode == "unsupported" {
				if !errors.Is(err, provider.ErrUnsupported) || calls.Load() != 1 {
					t.Fatal("unavailable filtering called provider", err)
				}
			} else if err != nil || calls.Load() != 2 {
				t.Fatal("valid empty boolean catalog failed", err)
			}
			var missing CollectionNotFoundError
			if _, err = s.Queryables("missing"); !errors.As(err, &missing) {
				t.Fatal("missing collection error lost", err)
			}
		})
	}
}
func TestQueryablesFilterValidationBeforeProvider(t *testing.T) {
	layer := serviceQueryableLayer(t, []provider.FeatureQueryable{{Name: "n", Type: provider.QueryableInteger}, {Name: "s", Type: provider.QueryableString}, {Name: "space name", Type: provider.QueryableString}})
	var calls atomic.Int32
	var received provider.FeatureQuery
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(func(_ context.Context, _ string, q provider.FeatureQuery, _ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		calls.Add(1)
		received = q
		return provider.FeatureQueryResult{}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"unknown = 1", "n = 'wrong'", "s = 2"} {
		expression, err := cql2.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1, Filter: &expression})
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) || invalid.Field != "filter" || calls.Load() != 0 {
			t.Fatal("invalid filter reached provider", err)
		}
	}
	expression, _ := cql2.Parse("n > 0.5 AND s IS NOT NULL")
	q := provider.FeatureQuery{Limit: 3, Offset: 7, IDs: []uint64{5}, Fields: []string{"s"}, Filter: &expression}
	original := expression.Root()
	if _, err := s.QueryCollectionPage(context.Background(), "public", q); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || !reflect.DeepEqual(received, q) || !reflect.DeepEqual(expression.Root(), original) || received.Filter == q.Filter {
		t.Fatal("query/filter forwarding ownership failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.QueryCollectionPage(ctx, "public", q); !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatal("cancellation lost", err)
	}
}
