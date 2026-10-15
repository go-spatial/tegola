//go:build cgo

package server

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type phase09NoTimeLayer struct{ provider.LayerInfo }

func (phase09NoTimeLayer) FeatureQuerySupported() error { return nil }
func (phase09NoTimeLayer) FeatureSourceSRID() uint64    { return 4326 }
func (phase09NoTimeLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}
func (phase09NoTimeLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}
func (layer phase09NoTimeLayer) FeatureQueryables() (provider.FeatureQueryables, error) {
	return layer.LayerInfo.(provider.FeatureQueryableLayerInfo).FeatureQueryables()
}

type phase09TemporalObserver struct {
	provider.FeatureQuerier
	calls      atomic.Int64
	unexpected atomic.Bool
}

func (observer *phase09TemporalObserver) QueryFeatures(ctx context.Context, layer string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	observer.calls.Add(1)
	if query.Temporal != nil {
		observer.unexpected.Store(true)
	}
	return observer.FeatureQuerier.QueryFeatures(ctx, layer, query, fn)
}

func TestFeatureRepresentationProtocolNoTemporalGPKG(t *testing.T) {
	p := filterProtocolGPKG(t)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	observer := &phase09TemporalObserver{FeatureQuerier: p}
	srv := phase09Server(t, []features.CollectionSource{{ID: "untimed", Layer: phase09NoTimeLayer{layers[0]}, Querier: observer}}, FeatureAPIConfig{DefaultLimit: 3, MaxLimit: 3}, "/nested/proxy")
	base := "/nested/proxy/features/collections/untimed/items?filter=" + url.QueryEscape("n = 7")
	for _, datetime := range []string{"2100-01-01T00:00:00Z", "2099-01-01T00:00:00Z/2101-01-01T00:00:00Z"} {
		r := protocolRequest(t, srv, "GET", base+"&datetime="+url.QueryEscape(datetime))
		var page struct {
			Features []struct {
				ID uint64 `json:"id"`
			} `json:"features"`
			Matched *uint64 `json:"numberMatched"`
		}
		if r.status != 200 || json.Unmarshal(r.body, &page) != nil {
			t.Fatalf("valid untimed request status%d", r.status)
		}
		var ids []uint64
		for _, feature := range page.Features {
			ids = append(ids, feature.ID)
		}
		if !slices.Equal(ids, []uint64{10, 20, 30}) || page.Matched == nil || *page.Matched != 3 {
			t.Fatal("untimed fixed membership/count differs")
		}
	}
	if observer.calls.Load() != 2 || observer.unexpected.Load() {
		t.Fatal("untimed provider received temporal predicate")
	}
	for _, datetime := range []string{"invalid", "2101-01-01T00:00:00Z/2099-01-01T00:00:00Z"} {
		r := protocolRequest(t, srv, "GET", base+"&datetime="+url.QueryEscape(datetime))
		if r.status != 400 || observer.calls.Load() != 2 {
			t.Fatal("invalid datetime reached provider")
		}
	}
}

func TestFeatureRepresentationProtocolRealGPKG(t *testing.T) {
	p := filterProtocolGPKG(t)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	srv := phase09Server(t, []features.CollectionSource{{ID: "data", Layer: layers[0], Querier: p}}, FeatureAPIConfig{DefaultLimit: 1, MaxLimit: 3}, "/nested/proxy")
	base := "/nested/proxy/features/collections/data/items"
	for _, tc := range []struct {
		name, filter, extra string
		ids                 []uint64
		known               bool
		matched             uint64
		next                bool
	}{
		{"first", "n = 7", "&limit=1", []uint64{10}, false, 0, true},
		{"second", "n = 7", "&limit=1&offset=1", []uint64{20}, false, 0, true},
		{"exhausted", "n = 7", "&limit=1&offset=2", []uint64{30}, true, 3, false},
		{"NULL", "n IS NULL", "&bbox=0,0,1,1", []uint64{40}, true, 1, false},
		{"exact-composition", "n = 7", "&bbox=14,29,17,32&datetime=1970-01-01T00:00:00Z", []uint64{10}, true, 1, false},
		{"literal-injection", "s = 'x'' OR TRUE -- ;'", "", []uint64{50}, true, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := base + "?filter=" + url.QueryEscape(tc.filter) + tc.extra
			jsonResponse := protocolRequest(t, srv, "GET", path+"&f=json")
			htmlResponse := protocolRequest(t, srv, "GET", path+"&f=html")
			if jsonResponse.status != 200 || htmlResponse.status != 200 {
				t.Fatalf("JSON/HTML status%d/%d", jsonResponse.status, htmlResponse.status)
			}
			details, anchors := phase09HTML(t, htmlResponse.body)
			var page struct {
				Features []struct {
					ID         uint64         `json:"id"`
					Properties map[string]any `json:"properties"`
				} `json:"features"`
				Matched *uint64                      `json:"numberMatched"`
				Links   []struct{ Rel, Href string } `json:"links"`
			}
			if err := json.Unmarshal([]byte(details), &page); err != nil {
				t.Fatal(err)
			}
			var ids []uint64
			for _, f := range page.Features {
				ids = append(ids, f.ID)
				for _, key := range []string{"n", "s", "b"} {
					if _, ok := f.Properties[key]; !ok {
						t.Fatal("selected property absent")
					}
				}
				if f.ID == 40 {
					for _, key := range []string{"n", "s", "b"} {
						if f.Properties[key] != nil {
							t.Fatal("NULL source value fabricated")
						}
					}
				}
			}
			if !slices.Equal(ids, tc.ids) || !slices.Equal(protocolIDs(t, jsonResponse), tc.ids) {
				t.Fatalf("static membership changed %v", ids)
			}
			if tc.known {
				if page.Matched == nil || *page.Matched != tc.matched {
					t.Fatal("exhausted exact count lost")
				}
			} else if page.Matched != nil {
				t.Fatal("lookahead incorrectly claimed exact count")
			}
			next := false
			for _, link := range page.Links {
				if link.Rel == "next" {
					next = true
					u, err := url.Parse(link.Href)
					if err != nil || u.Query().Get("filter") != tc.filter || u.Query().Get("f") != "html" || !strings.HasPrefix(u.Path, "/nested/proxy/features/") {
						t.Fatal("HTML paging lost filter/format/prefix")
					}
					if !slices.Contains(anchors, link.Href) {
						t.Fatal("genuine paging link not rendered anchor")
					}
				}
			}
			if next != tc.next {
				t.Fatal("exact paging state changed")
			}
			if htmlResponse.header.Get("Content-Crs") != jsonResponse.header.Get("Content-Crs") || htmlResponse.header.Get("Cache-Control") != "no-store" {
				t.Fatal("spatial/cache headers changed across representation")
			}
			head := protocolRequest(t, srv, "HEAD", path+"&f=html")
			if head.status != 200 || len(head.body) != 0 || head.header.Get("Content-Length") != htmlResponse.header.Get("Content-Length") {
				t.Fatal("realGPKG HTML HEAD mismatch")
			}
		})
	}
	// Excluded literal corruption proves the successful filtered pipeline did
	// not decode the sentinel before pushdown; selecting it still errors.
	bad := protocolRequest(t, srv, "GET", base+"?filter=n%20%3D%20999&f=html")
	if bad.status != 500 || bad.header.Get("Content-Type") != "application/json" || strings.Contains(string(bad.body), "source-integrity-sentinel") {
		t.Fatal("source corruption not genericJSONerror")
	}
}
