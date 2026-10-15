package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

func phase09Request(t *testing.T, srv *httptest.Server, method, path string, headers http.Header) protocolResponse {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A literal stable authority lets independent byte-cap fixtures use exact
	// response lengths without depending on randomized listener port lengths.
	req.Host = "fixture.example"
	req.Header = headers.Clone()
	client := *srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return protocolResponse{r.StatusCode, r.Header, b}
}

func TestFeatureHardeningProtocolAcceptanceHeadersAndBounds(t *testing.T) {
	old := Headers
	Headers = map[string]string{"ETag": "configured-stale", "Last-Modified": "Wed, 21 Oct 2015 07:28:00 GMT", "Vary": "Origin", "Access-Control-Allow-Origin": "https://allowed.example", "Access-Control-Allow-Credentials": "true", "X-Fixture": "retained"}
	t.Cleanup(func() { Headers = old })
	p := &phase09Query{text: "literal"}
	srv := phase09Server(t, []features.CollectionSource{{ID: "full", Layer: phase09FullLayer{}, Querier: p}}, FeatureAPIConfig{}, "/proxy/nested")
	base := "/proxy/nested/features"
	for _, tc := range []struct {
		method, path string
		headers      http.Header
		status       int
		preIO        bool
	}{
		{"GET", base + "/collections/full/items", http.Header{"If-None-Match": {"configured-stale"}}, 200, false},
		{"HEAD", base + "/collections/full/items?f=html", nil, 200, false},
		{"OPTIONS", base + "/collections/full/items", nil, 200, true},
		{"POST", base + "/collections/full/items", nil, 405, true},
		{"GET", base + "/collections/full/items?unknown=x", nil, 400, true},
		{"HEAD", base + "/missing", nil, 404, true},
		{"GET", base + "/", nil, 301, true},
		{"HEAD", base + "/collections/full/items?x=" + strings.Repeat("a", 65535), nil, 414, true},
		{"GET", base + "/collections/full/items?x=" + strings.Repeat("a", 65534), nil, 400, true},
		{"GET", base + "/collections/full/items", http.Header{"Accept": {"application/geo+json;q=1;pad=" + strings.Repeat("a", 16384-len("application/geo+json;q=1;pad="))}}, 200, false},
		{"HEAD", base + "/collections/full/items", http.Header{"Accept": {"application/geo+json;q=1;pad=" + strings.Repeat("a", 16385-len("application/geo+json;q=1;pad="))}}, 431, true},
	} {
		before := p.calls.Load()
		r := phase09Request(t, srv, tc.method, tc.path, tc.headers)
		if r.status != tc.status {
			t.Fatalf("%s expected%d got%d", tc.method, tc.status, r.status)
		}
		if tc.preIO && p.calls.Load() != before {
			t.Fatal("guard/method reached provider")
		}
		if tc.method == "HEAD" && len(r.body) != 0 {
			t.Fatal("HEAD body emitted")
		}
		if r.header.Get("Cache-Control") != "no-store" || r.header.Get("ETag") != "" || r.header.Get("Last-Modified") != "" {
			t.Fatal("feature validators/cache policy violated")
		}
		if r.header.Get("X-Fixture") != "retained" || r.header.Get("Access-Control-Allow-Origin") != "https://allowed.example" || r.header.Get("Access-Control-Allow-Credentials") != "true" {
			t.Fatal("configured headers lost")
		}
		if !strings.Contains(strings.Join(r.header.Values("Vary"), ","), "Accept") || !strings.Contains(strings.Join(r.header.Values("Vary"), ","), "Origin") {
			t.Fatal("Vary not merged")
		}
		if r.header.Get("Access-Control-Allow-Methods") != "GET, HEAD, OPTIONS" || !strings.Contains(strings.Join(r.header.Values("Access-Control-Expose-Headers"), ","), "Content-Crs") {
			t.Fatal("CORS feature methods/expose not truthful")
		}
		if (tc.method == "OPTIONS" || tc.method == "POST") && r.header.Get("Allow") != "GET, HEAD, OPTIONS" {
			t.Fatal("Allow not deterministic")
		}
	}
}

func TestFeatureHardeningProtocolAcceptanceEncodedSize(t *testing.T) {
	// Multibyte values and JSON HTML escaping make byte lengths different from
	// property rune counts. The original complete encoding determines boundary.
	p := &phase09Query{text: strings.Repeat("é<&", 1000)}
	sources := []features.CollectionSource{{ID: "full", Layer: phase09FullLayer{}, Querier: p}}
	large := phase09Server(t, sources, FeatureAPIConfig{}, "/")
	for _, format := range []string{"json", "html"} {
		t.Run(format, func(t *testing.T) {
			path := "/features/collections/full/items/18446744073709551615?f=" + format
			baseline := phase09Request(t, large, "GET", path, nil)
			if baseline.status != 200 || len(baseline.body) < 1024 {
				t.Fatal("size fixture too small")
			}
			n := int64(len(baseline.body))
			for _, delta := range []int64{-1, 0, 1} {
				srv := phase09Server(t, sources, FeatureAPIConfig{MaxResponseBytes: n + delta}, "/")
				get := phase09Request(t, srv, "GET", path, nil)
				head := phase09Request(t, srv, "HEAD", path, nil)
				want := 200
				if delta < 0 {
					// Over-budget responses are client-actionable: 400, not 500.
					want = 400
				}
				if get.status != want || head.status != want || head.header.Get("Content-Length") != get.header.Get("Content-Length") || len(head.body) != 0 {
					t.Fatal("encoding cap/HEAD decision differs")
				}
				if want == 200 {
					if string(get.body) != string(baseline.body) || get.header.Get("Content-Length") != strconv.Itoa(len(baseline.body)) {
						t.Fatal("accepted complete response changed")
					}
				} else {
					if get.header.Get("Content-Type") != "application/json" || get.header.Get("Content-Crs") != "" || !strings.Contains(string(get.body), `"code":"ResponseTooLarge"`) || strings.Contains(string(get.body), "é") {
						t.Fatal("overflow leaked partial success")
					}
				}
			}
		})
	}
}

type phase09BlockingQuery struct {
	entered chan struct{}
	exited  chan error
}

func (p *phase09BlockingQuery) QueryFeatures(ctx context.Context, _ string, _ provider.FeatureQuery, _ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	close(p.entered)
	<-ctx.Done()
	p.exited <- ctx.Err()
	return provider.FeatureQueryResult{}, ctx.Err()
}

func TestFeatureHardeningProtocolAcceptanceTimeoutAndClientCancellation(t *testing.T) {
	for _, clientCancel := range []bool{false, true} {
		t.Run(strconv.FormatBool(clientCancel), func(t *testing.T) {
			p := &phase09BlockingQuery{entered: make(chan struct{}), exited: make(chan error, 1)}
			timeout := 50 * time.Millisecond
			if clientCancel {
				timeout = 5 * time.Second
			}
			srv := phase09Server(t, []features.CollectionSource{{ID: "core", Layer: protocolLayer{}, Querier: p}}, FeatureAPIConfig{QueryTimeout: timeout}, "/")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/features/collections/core/items?f=html", nil)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				r, err := srv.Client().Do(req)
				if err != nil {
					done <- err
					return
				}
				defer r.Body.Close()
				b, err := io.ReadAll(r.Body)
				if err == nil && (r.StatusCode != 408 || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Cache-Control") != "no-store" || !strings.Contains(string(b), `"code":"RequestTimeout"`)) {
					err = errors.New("timeout response differs from genericJSONpolicy")
				}
				done <- err
			}()
			select {
			case <-p.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("provider never entered")
			}
			if clientCancel {
				cancel()
			}
			select {
			case err := <-p.exited:
				if clientCancel && !errors.Is(err, context.Canceled) {
					t.Fatal("client cancellation lost")
				}
				if !clientCancel && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("publication deadline lost")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cooperative provider not joined")
			}
			select {
			case err := <-done:
				if clientCancel && !errors.Is(err, context.Canceled) {
					t.Fatal("client request cancellation chain lost")
				}
				if !clientCancel && err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("HTTP operation not joined")
			}
		})
	}
}
