package server

import (
	"context"
	"encoding/json"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

type filterAcceptanceLayer struct{ protocolLayer }

func (filterAcceptanceLayer) FeatureQueryables() (provider.FeatureQueryables, error) {
	return provider.NewFeatureQueryables([]provider.FeatureQueryable{
		{Name: "b", Type: provider.QueryableBoolean, Nullable: true},
		{Name: "n", Type: provider.QueryableInteger, Nullable: true},
		{Name: "s", Type: provider.QueryableString, Nullable: true},
	})
}

type filterAcceptanceProbe struct {
	calls atomic.Int64
	err   error
}

func (p *filterAcceptanceProbe) QueryFeatures(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	p.calls.Add(1)
	return provider.FeatureQueryResult{}, p.err
}

func TestFeatureFilterProtocolRejectsBeforeProvider(t *testing.T) {
	p := &filterAcceptanceProbe{}
	srv := protocolServer(t, p, filterAcceptanceLayer{}, "/stage")
	for _, query := range []string{
		"filter=", "filter-lang=cql2-text", "filter=TRUE&filter=FALSE",
		"filter=TRUE&filter-lang=cql2-json", "filter=TRUE&filter-crs=invalid",
		"filter=" + url.QueryEscape("private_id = 1"),
		"filter=" + url.QueryEscape("id = 1"),
		"filter=" + url.QueryEscape("n = '7'"),
		"filter=" + url.QueryEscape("s LIKE 'x'"),
		"filter=" + url.QueryEscape("n = 1; DROP TABLE items"),
	} {
		t.Run(query, func(t *testing.T) {
			r := protocolRequest(t, srv, "GET", "/stage/features/collections/public/items?"+query)
			if r.status != 400 || p.calls.Load() != 0 || r.header.Get("Cache-Control") != "no-store" {
				t.Fatalf("rejection status=%d provider calls=%d", r.status, p.calls.Load())
			}
		})
	}
	r := protocolRequest(t, srv, "GET", "/stage/features/collections/public/queryables", "application/schema+json")
	var schema struct {
		Properties           map[string]json.RawMessage `json:"properties"`
		AdditionalProperties *bool                      `json:"additionalProperties"`
	}
	if err := json.Unmarshal(r.body, &schema); err != nil {
		t.Fatal(err)
	}
	if r.status != 200 || len(schema.Properties) != 3 || schema.AdditionalProperties == nil || *schema.AdditionalProperties || p.calls.Load() != 0 {
		t.Fatal("queryables discovery invoked feature I/O or changed closed public schema")
	}
}

func TestFeatureFilterProtocolContextClassification(t *testing.T) {
	p := &filterAcceptanceProbe{err: context.Canceled}
	srv := protocolServer(t, p, filterAcceptanceLayer{}, "/")
	r := protocolRequest(t, srv, "GET", "/features/collections/public/items?filter=TRUE")
	if r.status != 408 || p.calls.Load() != 1 || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("context status=%d provider calls=%d", r.status, p.calls.Load())
	}
}
