package cache_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache"
)

func TestParseKey(t *testing.T) {
	testcases := []struct {
		input    string
		expected *cache.Key
	}{
		{
			input: "/12/11/123",
			expected: &cache.Key{
				Z: 12,
				X: 11,
				Y: 123,
			},
		},
		{
			input: "/osm/12/11/123",
			expected: &cache.Key{
				Z:       12,
				X:       11,
				Y:       123,
				MapName: "osm",
			},
		},
		{
			input: "/osm/buildings/12/11/123",
			expected: &cache.Key{
				Z:         12,
				X:         11,
				Y:         123,
				MapName:   "osm",
				LayerName: "buildings",
			},
		},
	}

	for i, tc := range testcases {
		output, err := cache.ParseKey(tc.input)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("testcase (%v) failed. expected (%+v) does not match output (%+v)", i, tc.expected, output)
			continue
		}
	}
}
