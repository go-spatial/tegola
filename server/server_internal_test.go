package server

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
)

func TestHostName(t *testing.T) {
	// Helper function to set up table tests.
	urlFromString := func(urlString string) *url.URL {
		url, err := url.Parse(urlString)
		if err != nil {
			t.Errorf("Could not create url.URL from %v: %v", urlString, err)
		}
		return url
	}

	// Minimal http.Request with only URL & Host properties set
	mockRequest := func(u *url.URL) http.Request {
		r := http.Request{URL: u, Host: u.Host}
		return r
	}

	testcases := []struct {
		request  http.Request
		hostName string
		port     string
		expected string
	}{
		{
			// With hostname & port unset in config, expect host:port matching URL
			request:  mockRequest(urlFromString("http://localhost:8080/capabilities")),
			expected: "localhost:8080",
		},
		{
			// With hostname set and port set to "none" in config, expect "cdn.tegola.io"
			request:  mockRequest(urlFromString("http://localhost:8080/capabilities")),
			hostName: "cdn.tegola.io",
			port:     "none",
			expected: "cdn.tegola.io",
		},
		{
			// Hostname set, no port in config, but port in url.  Expect <config_host>:<url_port>.
			request:  mockRequest(urlFromString("http://localhost:8080/capabilities")),
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io:8080",
		},
		{
			// Hostname set, no port in config or url, expect hostname to match config.
			request:  mockRequest(urlFromString("http://localhost/capabilities")),
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
		{
			// Hostname unset, no port in config or url, expect hostname to match url host.
			request:  mockRequest(urlFromString("http://localhost/capabilities")),
			expected: "localhost",
		},
	}

	for i, tc := range testcases {
		//	set the package variable
		HostName = tc.hostName
		Port = tc.port

		output := hostName(&tc.request)
		if output != tc.expected {
			t.Errorf("testcase (%v) failed. expected (%v) does not match result (%v)", i, tc.expected, output)
		}
	}
}

func TestScheme(t *testing.T) {
	testcases := []struct {
		request  http.Request
		expected string
	}{
		{
			request:  http.Request{},
			expected: "http",
		},
		{
			request: http.Request{
				TLS: &tls.ConnectionState{},
			},
			expected: "https",
		},
		{
			request: http.Request{
				Header: map[string][]string{
					"X-Forwarded-Proto": []string{
						"https",
						"http",
					},
				},
			},
			expected: "https",
		},
	}

	for i, tc := range testcases {
		output := scheme(&tc.request)
		if output != tc.expected {
			t.Errorf("testcase (%v) failed. expected (%v) does not match result (%v)", i, tc.expected, output)
		}
	}
}
