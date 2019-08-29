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
		"hostname set port in request": {
			// Hostname set, no port in config, but port in url.  Expect <config_host>:<url_port>.
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
		request  http.Request
		expected string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			output := scheme(&tc.request)
			if output != tc.expected {
				t.Errorf("scheme, expected (%v) got (%v)", tc.expected, output)
			}
		}
	}

	tests := map[string]tcase{
		"http": {
			request:  http.Request{},
			expected: "http",
		},
		"https": {
			request: http.Request{
				TLS: &tls.ConnectionState{},
			},
			expected: "https",
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
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestBuildCapabilitiesURL(t *testing.T) {
	type tcase struct {
		request   http.Request
		uriParts  []string
		uriPrefix string
		query     url.Values
		expected  string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {

			if tc.uriPrefix != "" {
				URIPrefix = tc.uriPrefix
			} else {
				URIPrefix = "/"
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
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

	// reset the URIPrefix. Ideally this would not be necessary but the server package is
	// designed as a singleton right now. Eventually this will change so the tests
	// don't need to consider each other
	URIPrefix = "/"
}
