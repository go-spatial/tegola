package server

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestHostName(t *testing.T) {
	testcases := []struct {
		request  http.Request // request passed to server.hostName()
		hostName string       // config file hostname
		port     string       // config file port
		expected string
	}{
		// With no host or port set in config, the hostname should match that used in request uri
		{
			request: http.Request{
				Host: "localhost",
			},
			hostName: "",
			expected: "localhost",
		},
		// With a hostname set in config, that's what the resulting hostname should equal
		{
			request:  http.Request{},
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
		// With a hostname set in config and port set to "none", resulting hostname should match
		//	config hostname.
		{
			request: http.Request{
				Host: "localhost:8080",
			},
			hostName: "cdn.tegola.io",
			port:     "none",
			expected: "cdn.tegola.io",
		},
		// With a hostname set in config, no port set in config, and an alternative port set
		//	in request uri, the resulting hostname should be <configHostName>:<requestPort>
		{
			request: http.Request{
				Host: "localhost:8080",
			},
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io:8080",
		},
		// With hostname and port set in config, the result should be <configHostName>:<configPort>
		{
			request: http.Request{
				Host: "localhost:8080",
			},
			hostName: "cdn.tegola.io",
			port:     "877",
			expected: "cdn.tegola.io:877",
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
