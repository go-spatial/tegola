package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/ogc/features"
)

// Extract the fixed embedded template's details/navigation using only stdlib.
// Executable DOM behavior is independently checked in the browser fixture.
func phase09HTML(t *testing.T, body []byte) (string, []string) {
	t.Helper()
	text := string(body)
	if regexp.MustCompile(`(?i)<(?:script|iframe|img)(?:\s|>)`).MatchString(text) || regexp.MustCompile(`(?i)<[^>]+\son[a-z]+\s*=`).MatchString(text) {
		t.Fatal("executable source markup emitted")
	}
	pre := regexp.MustCompile(`(?s)<pre\b[^>]*>(.*?)</pre>`).FindStringSubmatch(text)
	if len(pre) != 2 {
		t.Fatal("complete response details missing")
	}
	var links []string
	for _, m := range regexp.MustCompile(`<a\b[^>]*href="([^"]+)"`).FindAllStringSubmatch(text, -1) {
		links = append(links, html.UnescapeString(m[1]))
	}
	return html.UnescapeString(pre[1]), links
}
func TestFeatureHTMLProtocolAcceptanceNegotiation(t *testing.T) {
	p := &phase09Query{text: "literal"}
	srv := phase09Server(t, []features.CollectionSource{{ID: "full", Layer: phase09FullLayer{}, Querier: p}}, FeatureAPIConfig{}, "/proxy/nested")
	base := "/proxy/nested/features"
	for _, tc := range []struct {
		name, path, accept, media string
		status                    int
	}{
		{"default", base + "/collections/full/items", "", "application/geo+json", 200},
		{"html", base + "/collections/full/items", "text/html", "text/html; charset=utf-8", 200},
		{"tieJSON", base + "/collections/full/items", "text/html, application/geo+json", "application/geo+json", 200},
		{"excludeJSON", base + "/collections/full/items", "application/geo+json;q=0,*/*;q=1", "text/html; charset=utf-8", 200},
		{"excludeBoth", base + "/collections/full/items", "application/geo+json;q=0,text/html;q=0,*/*;q=1", "application/json", 406},
		{"overrideHTML", base + "/collections/full/items?f=html", "application/xml", "text/html; charset=utf-8", 200},
		{"overrideJSON", base + "/collections/full/items?f=json", "text/html", "application/geo+json", 200},
		{"invalidFormat", base + "/collections/full/items?f=xml", "text/html", "application/json", 400},
		{"repeatFormat", base + "/collections/full/items?f=html&f=json", "text/html", "application/json", 400},
		{"emptyFormat", base + "/collections/full/items?f=", "text/html", "application/json", 400},
		{"malformedQ", base + "/collections/full/items", "text/html;q=1.001", "application/json", 406},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := p.calls.Load()
			accept := []string(nil)
			if tc.accept != "" {
				accept = []string{tc.accept}
			}
			get := protocolRequest(t, srv, "GET", tc.path, accept...)
			if get.status != tc.status || get.header.Get("Content-Type") != tc.media {
				t.Fatalf("status/media %d %s", get.status, get.header.Get("Content-Type"))
			}
			head := protocolRequest(t, srv, "HEAD", tc.path, accept...)
			if head.status != get.status || head.header.Get("Content-Type") != get.header.Get("Content-Type") || head.header.Get("Content-Length") != get.header.Get("Content-Length") || len(head.body) != 0 {
				t.Fatal("selected representation HEAD parity")
			}
			if tc.status != 200 && p.calls.Load() != before {
				t.Fatal("invalid negotiation reached provider")
			}
		})
	}
}

func TestFeatureHTMLProtocolAcceptanceEscapingAndLinks(t *testing.T) {
	hostile := `</pre><script>window.phase9Pwned=1</script><img src=x onerror="window.phase9Pwned=2"> & "quotes"`
	p := &phase09Query{text: hostile}
	srv := phase09Server(t, []features.CollectionSource{{ID: "full", Title: hostile, Description: hostile, Layer: phase09FullLayer{}, Querier: p}}, FeatureAPIConfig{Title: hostile, Description: hostile}, "/proxy/nested")
	base := "/proxy/nested/features"
	for _, suffix := range []string{"", "/api", "/conformance", "/collections", "/collections/full", "/collections/full/queryables", "/collections/full/items", "/collections/full/items/18446744073709551615"} {
		t.Run(fmt.Sprintf("resource%s", suffix), func(t *testing.T) {
			r := protocolRequest(t, srv, "GET", base+suffix+"?f=html")
			if r.status != 200 || r.header.Get("Content-Type") != "text/html; charset=utf-8" {
				t.Fatalf("HTML resource%d %s", r.status, r.body)
			}
			details, anchors := phase09HTML(t, r.body)
			var model any
			decoder := json.NewDecoder(strings.NewReader(details))
			decoder.UseNumber()
			if err := decoder.Decode(&model); err != nil {
				t.Fatal("HTML details are not complete JSON", err)
			}
			if len(anchors) == 0 {
				t.Fatal("protocol navigation missing")
			}
			// Read only known protocol-link locations, never feature properties.
			known := map[string]int{}
			add := func(v any) {
				if values, ok := v.([]any); ok {
					for _, raw := range values {
						link := phase09Object(t, raw)
						href, ok := link["href"].(string)
						if !ok {
							t.Fatal("protocol link malformed")
						}
						known[href]++
					}
				}
			}
			root := phase09Object(t, model)
			add(root["links"])
			if collections, ok := root["collections"].([]any); ok {
				for _, raw := range collections {
					add(phase09Object(t, raw)["links"])
				}
			}
			actual := map[string]int{}
			for _, href := range anchors {
				actual[href]++
			}
			for href, n := range known {
				if actual[href] != n {
					t.Fatalf("genuine protocol anchor multiplicity%s expected%d got%d", href, n, actual[href])
				}
			}
			for _, href := range anchors {
				u, err := url.Parse(href)
				if err != nil || u.Host == "untrusted.invalid" || !strings.HasPrefix(u.Path, base) {
					t.Fatalf("property/external link treated as navigation %s", href)
				}
				if f := u.Query().Get("f"); f != "json" && f != "html" {
					t.Fatalf("browser link format missing %s", href)
				}
			}
			if strings.Contains(suffix, "/items") {
				object := phase09Object(t, model)
				if strings.HasSuffix(suffix, "/items") {
					object = phase09Object(t, object["features"].([]any)[0])
				}
				props := phase09Object(t, object["properties"])
				if props["s"] != hostile || props["n"] != nil || props["b"] != false {
					t.Fatal("HTML data changed null/false/hostile string")
				}
				if object["id"] != json.Number("18446744073709551615") {
					t.Fatal("feature omitted")
				}
			}
		})
	}
}
