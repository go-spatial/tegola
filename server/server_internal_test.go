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

	fn := func(t *testing.T, tc tcase) {
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
			expected: "cdn.tegola.io:8080",
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
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestScheme(t *testing.T) {
	type tcase struct {
		request  http.Request
		expected string
	}

	fn := func(t *testing.T, tc tcase) {
		output := scheme(&tc.request)
		if output != tc.expected {
			t.Errorf("scheme, expected (%v) got (%v)", tc.expected, output)
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
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
