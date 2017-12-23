package server

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
)

func TestHostName(t *testing.T) {
	testcases := []struct {
		url      string
		hostName string
		port     int
		expected string
	}{
		// With no host or port set in config, the hostname should match that used in request uri
		{
			// With hostname & port unset in config, expect host:port matching URL
			url:      "http://localhost:8080/capabilities",
			expected: "localhost:8080",
		},
		// With a hostname set in config, that's what the resulting hostname should equal
		{
			// With hostname set and negative port in config, expect "cdn.tegola.io" despite url port
			url:      "http://localhost:8080/capabilities",
			port:     -1,
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
		{
			// With hostname and port set in config, expect <config_host>:<config_port>
			url:      "http://localhost:8080/capabilities",
			port:     9000,
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io:9000",
		},
		{
			// Hostname set, no port in config, but port in url.  Expect <config_host>:<url_port>.
			url:      "http://localhost:8080/capabilities",
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io:8080",
		},
		{
			// Hostname set, no port in config or url, expect hostname to match config.
			url:      "http://localhost/capabilities",
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
		{
			// Hostname unset, no port in config or url, expect hostname to match url host.
			url:      "http://localhost/capabilities",
			expected: "localhost",
		},
	}

	for i, tc := range testcases {
		//	set the package variable
		HostName = tc.hostName
		Port = tc.port

		url, err := url.Parse(tc.url)
		if err != nil {
			t.Errorf("testcase (%v) failed. could not create url.URL from (%v): %v", i, tc.url, err)
		}

		req := http.Request{URL: url, Host: url.Host}

		output := hostName(&req)
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
