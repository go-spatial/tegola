package server

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestHostName(t *testing.T) {
	testcases := []struct {
		request  http.Request
		hostName string
		expected string
	}{
		{
			request: http.Request{
				Host: "localhost",
			},
			hostName: "",
			expected: "localhost",
		},
		{
			request:  http.Request{},
			hostName: "cdn.tegola.io",
			expected: "cdn.tegola.io",
		},
	}

	for i, tc := range testcases {
		//	set the package variable
		HostName = tc.hostName

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
