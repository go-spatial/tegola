//go:build cgo

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

type snapshotReviewProvider struct {
	*gpkg.Provider
	changed bool
	fail    bool
}

func (p *snapshotReviewProvider) CurrentRevision(context.Context, string, uint64) (string, error) {
	if p.fail {
		return "", errors.New("revision storage unavailable")
	}
	if p.changed {
		return "0.2", nil
	}
	return "0.1", nil
}

func (p *snapshotReviewProvider) QueryFeatures(ctx context.Context, layer string, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	name := "old"
	if p.changed {
		name = "new"
	}
	err := fn(&provider.Feature{ID: 1, Geometry: geom.Point{10, 20}, SRID: 4326, Tags: map[string]interface{}{"name": name}})
	// Model a writer committing immediately after the read snapshot closes.
	p.changed = true
	return provider.FeatureQueryResult{NumberReturned: 1}, err
}

func TestItemETagMatchesReadSnapshot(t *testing.T) {
	_, gp := part4Service(t)
	layers, err := gp.Layers()
	if err != nil {
		t.Fatal(err)
	}
	p := &snapshotReviewProvider{Provider: gp}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	router := part4Router(t, part4API(t, svc))
	r := doRequest(t, router, http.MethodGet, "/features/collections/sites/items/1", "", nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"name":"new"`) || !strings.HasPrefix(r.Header().Get("ETag"), `"0.2.`) {
		t.Fatalf("inconsistent representation: %d %s %s", r.Code, r.Header().Get("ETag"), r.Body.String())
	}
	p.fail = true
	r = doRequest(t, router, http.MethodGet, "/features/collections/sites/items/1", "", nil)
	if r.Code != 500 || r.Header().Get("ETag") != "" {
		t.Fatalf("revision failure silently bypassed: %d %v", r.Code, r.Header())
	}
}

func TestCommittedRepresentationFailurePreservesSuccess(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 4}}
	receipt := provider.CommitReceipt{Status: provider.CommitCommitted, TransactionID: "confirmed-tx"}
	for _, value := range []any{strings.Repeat("x", 100), make(chan int)} {
		for _, status := range []int{http.StatusCreated, http.StatusOK} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/features/collections/sites/items", nil)
			api.writeMutationRepresentation(w, r, status, value, "0.1", receipt)
			want := http.StatusNoContent
			if status == http.StatusCreated {
				want = status
			}
			if w.Code != want || w.Header().Get("Tegola-Commit-Status") != "committed" || w.Header().Get("ETag") != "" {
				t.Fatalf("committed write misreported: %d %v", w.Code, w.Header())
			}
		}
	}
}

func TestCommittedCreateReadbackFailurePreservesLocation(t *testing.T) {
	_, gp := part4Service(t)
	layers, err := gp.Layers()
	if err != nil {
		t.Fatal(err)
	}
	p := &snapshotReviewProvider{Provider: gp, fail: true}
	svc, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	router := part4Router(t, part4API(t, svc))
	r := doRequest(t, router, http.MethodPost, "/features/collections/sites/items",
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[10,20]},"properties":{"name":"committed"}}`,
		map[string]string{"Content-Type": mediaGeoJSON})
	if r.Code != http.StatusCreated || !strings.Contains(r.Header().Get("Location"), "/items/1") || r.Header().Get("Tegola-Commit-Status") != "committed" {
		t.Fatalf("lost create receipt: %d %v %s", r.Code, r.Header(), r.Body.String())
	}
}
