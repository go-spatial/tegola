package server

import (
	"testing"

	"github.com/go-spatial/tegola"
)

func TestStringToColorHex(t *testing.T) {
	testcases := []struct {
		input    string
		expected string
	}{
		{
			input:    "alex rolek",
			expected: "#33ce8a",
		},
	}

	for i, tc := range testcases {
		output := stringToColorHex(tc.input)

		if tc.expected != output {
			t.Errorf("testcase (%v) failed. exected (%v) does not match output (%v)", i, tc.expected, output)
		}
	}
}

func TestHanleMapLayerChooseTileBuffer(t *testing.T) {
	// save current tile buffer value and restore after test
	currentTileBuffers := TileBuffers
	defer func() {
		TileBuffers = currentTileBuffers
	}()

	// redefine default tile buffer for map with name "test-tilebuffer"
	tileBuffers := map[string]float64{
		"test-tilebuffer": 32.0,
	}

	testcases := []struct {
		tileBuffers map[string]float64
		mapname     string
		expected    float64
	}{
		{
			tileBuffers: nil,
			mapname:     "nil-tilebuffers",
			expected:    tegola.DefaultTileBuffer,
		},
		{
			tileBuffers: make(map[string]float64),
			mapname:     "empty-tilebuffers",
			expected:    tegola.DefaultTileBuffer,
		},
		{
			tileBuffers: tileBuffers,
			mapname:     "default-tilebuffer",
			expected:    64.0,
		},
		{
			tileBuffers: tileBuffers,
			mapname:     "test-tilebuffer",
			expected:    32.0,
		},
	}

	for _, tc := range testcases {
		TileBuffers = tc.tileBuffers
		output := chooseTileBuffer(tc.mapname)
		if output != tc.expected {
			t.Fatalf("testcase (%v) failed. expected (%v) does not match output (%v)", tc.mapname, tc.expected, output)
		}
	}
}
