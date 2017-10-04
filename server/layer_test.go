package server_test

import (
	"testing"

	"github.com/terranodo/tegola/server"
)

func TestLayerMVTName(t *testing.T) {
	testcases := []struct {
		layer    server.Layer
		expected string
	}{
		{
			layer:    testLayer1,
			expected: "test-layer-1",
		},
		{
			layer:    testLayer2,
			expected: "test-layer-2-name",
		},
	}

	for i, tc := range testcases {
		output := tc.layer.MVTName()
		if output != tc.expected {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expected)
		}
	}
}
