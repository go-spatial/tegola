package atlas_test

import (
	"testing"

	"github.com/go-spatial/tegola/atlas"
)

func TestLayerMVTName(t *testing.T) {
	testcases := []struct {
		layer    atlas.Layer
		expected string
	}{
		{
			layer:    testLayer1,
			expected: "test-layer",
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
