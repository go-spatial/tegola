package server

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFeatureHTMLLinkMediaEssence(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{BasePath: "/features"}, uriPrefix: "/nested"}
	r := httptest.NewRequest("GET", "/nested/features", nil)
	parameters := url.Values{"offset": {"2"}}
	for _, relation := range []string{"self", "alternate", "next", "service-doc"} {
		link := api.formatLink(r, "/items", relation, "application/geo+json", parameters, "html")
		if link.Type != "text/html" || !strings.HasSuffix(link.Href, "/nested/features/items?f=html&offset=2") {
			t.Fatal("HTML link must use canonical media essence and retain parameters", link)
		}
	}
	links := api.representationLinks(r, "/items", "application/geo+json", parameters)
	if links[0].Type != "application/geo+json" || links[1].Type != "text/html" || parameters.Get("f") != "" {
		t.Fatal("alternate metadata or caller parameters changed", links, parameters)
	}
}

func TestFeatureHTMLCompleteEscapedModel(t *testing.T) {
	value := map[string]any{"id": uint64(18446744073709551615), "null": nil, "empty": "", "geometry": []int{1, 2, 3}, "properties": map[string]any{"links": []featureLink{{Href: "https://property.example/", Rel: "property"}}, "attack": "<script>alert(1)</script>"}}
	document := featureHTMLDocument{Title: "<script>title</script>", Description: "<img src=x onerror=alert(1)>", Value: value, Links: []featureLink{{Href: "/nested/features?f=json&limit=2", Rel: "alternate", Type: "application/json", Title: "<b>JSON</b>"}}}
	result, err := renderFeatureHTML(context.Background(), document)
	if err != nil {
		t.Fatal(err)
	}
	body := string(result)
	if strings.Contains(body, "<script>") || strings.Contains(body, "<img ") || strings.Contains(body, "<b>JSON") || strings.Count(body, "<a ") != 1 || strings.Contains(body, `href="https://property.example/"`) {
		t.Fatal("unescaped markup or property link became navigation", body)
	}
	if !strings.Contains(body, `href="/nested/features?f=json&amp;limit=2"`) {
		t.Fatal("genuine link missing", body)
	}
	start, end := strings.Index(body, "<pre>"), strings.Index(body, "</pre>")
	if start < 0 || end <= start {
		t.Fatal("complete model missing")
	}
	raw := html.UnescapeString(body[start+5 : end])
	want, _ := json.MarshalIndent(value, "", "  ")
	if raw != string(want) {
		t.Fatalf("model changed: %s", raw)
	}
	if document.Links[0].Href != "/nested/features?f=json&limit=2" {
		t.Fatal("input link mutated")
	}
}

type featureHTMLFailWriter struct {
	err   error
	short bool
}

func (w featureHTMLFailWriter) Write(p []byte) (int, error) {
	if w.short {
		return 0, nil
	}
	return 0, w.err
}

func TestFeatureHTMLWriterAndCancellationChains(t *testing.T) {
	document := featureHTMLDocument{Title: "Features", Value: map[string]any{"a": nil}}
	sentinel := errors.New("writer failed")
	if err := renderFeatureHTMLTo(context.Background(), featureHTMLFailWriter{err: sentinel}, document); !errors.Is(err, sentinel) {
		t.Fatalf("writer chain: %v", err)
	}
	if err := renderFeatureHTMLTo(context.Background(), featureHTMLFailWriter{short: true}, document); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write chain: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := renderFeatureHTML(ctx, document); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation chain: %v", err)
	}
	if err := renderFeatureHTMLTo(context.Background(), nil, document); err == nil {
		t.Fatal("nil writer accepted")
	}
	if _, err := renderFeatureHTML(context.Background(), featureHTMLDocument{Value: make(chan int)}); err == nil {
		t.Fatal("invalid model accepted")
	}
}
