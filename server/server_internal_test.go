package server

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
)

func TestHostName(t *testing.T) {
	type tcase struct {
		url      string
		hostName string
		port     string
		expected string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			// set the package variable
			HostName = tc.hostName
			Port = tc.port

			url, err := url.Parse(tc.url)
			if err != nil {
				t.Errorf("url(%v) parse error, expected nil got %v", tc.url, err)
				return
			}

			req := http.Request{URL: url, Host: url.Host}

			output := hostName(&req)
			if output != tc.expected {
				t.Errorf("hostname, expected %v got %v", tc.expected, output)
				return
			}

		}
	}

	tests := map[string]tcase{
		"no host or port set": {
			// With hostname & port unset in config, expect host:port matching URL
			url:      "http://localhost:8080/capabilities",
			expected: "localhost:8080",
		},
		"hostname set": {
			// With hostname set and port set to "none" in config, expect "cdn.tegola.io"
			url:      "http://localhost:8080/capabilities",
			hostName: "cdn.tegola.io",
			port:     "none",
			expected: "cdn.tegola.io",
		},
		"hostname set port set": {
			// With hostname set and port set to "none" in config, expect "cdn.tegola.io"
			url:      "http://localhost:8080/capabilities",
			hostName: "cdn.tegola.io",
			port:     ":9090",
			expected: "cdn.tegola.io",
		},
		"hostname set port in request": {
			// Hostname set, no port in config, but port in url.  Expect <config_host>
			url:      "http://localhost:8080/capabilities",
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
		"hostname set no port in config or url": {
			url:      "http://localhost/capabilities",
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
		"hostname unset no port in config or url": {
			url:      "http://localhost/capabilities",
			expected: "localhost",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestScheme(t *testing.T) {
	type tcase struct {
		request       http.Request
		proxyProtocol string
		expected      string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			ProxyProtocol = tc.proxyProtocol
			output := scheme(&tc.request)
			if output != tc.expected {
				t.Errorf("scheme, expected (%v) got (%v)", tc.expected, output)
			}
		}
	}

	tests := map[string]tcase{
		"http no proxyProtocol": {
			request:  http.Request{},
			expected: "http",
		},
		"http with http proxyProtocol": {
			request:       http.Request{},
			proxyProtocol: "http",
			expected:      "http",
		},
		"http with https proxyProtocol": {
			request:       http.Request{},
			proxyProtocol: "https",
			expected:      "https",
		},
		"https": {
			request: http.Request{
				TLS: &tls.ConnectionState{},
			},
			expected: "https",
		},
		"https with http proxyProtocol": {
			request: http.Request{
				TLS: &tls.ConnectionState{},
			},
			proxyProtocol: "http",
			expected:      "http",
		},
		"https with empty proxyProtocol": {
			request: http.Request{
				TLS: &tls.ConnectionState{},
			},
			proxyProtocol: "",
			expected:      "https",
		},
		"x-forwarded-proto": {
			request: http.Request{
				Header: map[string][]string{
					"X-Forwarded-Proto": {
						"https",
						"http",
					},
				},
			},
			expected: "https",
		},
		"x-forwarded-proto with http proxyProtocol": {
			request: http.Request{
				Header: map[string][]string{
					"X-Forwarded-Proto": {
						"https",
						"http",
					},
				},
			},
			proxyProtocol: "http",
			expected:      "http",
		},
		"http x-forwarded-proto with https proxyProtocol": {
			request: http.Request{
				Header: map[string][]string{
					"X-Forwarded-Proto": {
						"http",
					},
				},
			},
			proxyProtocol: "https",
			expected:      "https",
		},
		"https x-forwarded-proto with empty proxyProtocol": {
			request: http.Request{
				Header: map[string][]string{
					"X-Forwarded-Proto": {
						"https",
					},
				},
			},
			proxyProtocol: "",
			expected:      "https",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

	ProxyProtocol = ""
}

func TestBuildCapabilitiesURL(t *testing.T) {

	type tcase struct {
		request       http.Request
		uriParts      []string
		uriPrefix     string
		query         url.Values
		expected      string
		proxyProtocol string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			if tc.uriPrefix != "" {
				URIPrefix = tc.uriPrefix
			} else {
				URIPrefix = "/"
			}

			if tc.proxyProtocol != "" {
				ProxyProtocol = tc.proxyProtocol
			} else {
				ProxyProtocol = ""
			}

			output := buildCapabilitiesURL(&tc.request, tc.uriParts, tc.query)
			if output != tc.expected {
				t.Errorf("expected (%v) got (%v)", tc.expected, output)
			}
		}
	}

	tests := map[string]tcase{
		"no uri prefix no query": {
			request: http.Request{
				Host: "cdn.tegola.io",
			},
			uriParts: []string{"foo", "bar"},
			query:    url.Values{},
			expected: "http://cdn.tegola.io/foo/bar",
		},
		"uri prefix no query": {
			request: http.Request{
				Host: "cdn.tegola.io",
			},
			uriParts:  []string{"foo", "bar"},
			uriPrefix: "/tegola",
			query:     url.Values{},
			expected:  "http://cdn.tegola.io/tegola/foo/bar",
		},
		"uri prefix and query": {
			request: http.Request{
				Host: "cdn.tegola.io",
			},
			uriParts:  []string{"foo", "bar"},
			uriPrefix: "/tegola",
			query: url.Values{
				"debug": []string{"true"},
			},
			expected: "http://cdn.tegola.io/tegola/foo/bar?debug=true",
		},
		"http proxy_protocol": {
			request: http.Request{
				Host: "cdn.tegola.io",
			},
			uriParts:      []string{"foo", "bar"},
			query:         url.Values{},
			proxyProtocol: "http",
			expected:      "http://cdn.tegola.io/foo/bar",
		},
		"https proxy_protocol": {
			request: http.Request{
				Host: "cdn.tegola.io",
			},
			uriParts:      []string{"foo", "bar"},
			query:         url.Values{},
			proxyProtocol: "https",
			expected:      "https://cdn.tegola.io/foo/bar",
		},
		"http proxy_protocol with https Request": {
			request: http.Request{
				TLS:  &tls.ConnectionState{},
				Host: "cdn.tegola.io",
			},
			uriParts:      []string{"foo", "bar"},
			query:         url.Values{},
			proxyProtocol: "http",
			expected:      "http://cdn.tegola.io/foo/bar",
		},
		"https proxy_protocol with uri_prefix": {
			request: http.Request{
				Host: "cdn.tegola.io",
			},
			uriParts:      []string{"foo", "bar"},
			uriPrefix:     "/tegola",
			query:         url.Values{},
			proxyProtocol: "https",
			expected:      "https://cdn.tegola.io/tegola/foo/bar",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

	// reset the URIPrefix. Ideally this would not be necessary but the server package is
	// designed as a singleton right now. Eventually this will change so the tests
	// don't need to consider each other
	URIPrefix = "/"
	ProxyProtocol = ""
}
