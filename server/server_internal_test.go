package server

import (
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
		HostName = tc.hostName

		output := hostName(&tc.request)
		if output != tc.expected {
			t.Errorf("testcase (%v) failed. expected (%v) does not match result (%v)", i, tc.expected, output)
		}
	}
}
