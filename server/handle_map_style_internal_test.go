package server

import (
	"fmt"
	"testing"

	"github.com/alexeydott/tegola/mapbox/style"
)

func TestStringToColorHex(t *testing.T) {
	type tcase struct {
		input    string
		expected string
	}
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			output := stringToColorHex(tc.input)

			if tc.expected != output {
				t.Errorf("color hex. expected (%v) got (%v)", tc.expected, output)
			}
		}
	}
	testcases := []tcase{
		{
			input:    "alex rolek",
			expected: "#33ce8a",
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("%d %v", i, tc.input), fn(tc))
	}
}

func TestCollectionStyleLayers(t *testing.T) {
	layers := collectionStyleLayers("map", "mixed")
	if len(layers) != 3 {
		t.Fatalf("generated %d collection style layers, want 3", len(layers))
	}

	want := []struct {
		id        string
		layerType string
		geomType  string
	}{
		{"mixed-point", "circle", "Point"},
		{"mixed-line", "line", "LineString"},
		{"mixed-fill", "fill", "Polygon"},
	}
	for i, tc := range want {
		if layers[i].ID != tc.id {
			t.Errorf("layer %d ID = %q, want %q", i, layers[i].ID, tc.id)
		}
		if layers[i].Type != tc.layerType {
			t.Errorf("layer %d type = %q, want %q", i, layers[i].Type, tc.layerType)
		}
		if got := layers[i].Filter[2]; got != tc.geomType {
			t.Errorf("layer %d filter geometry = %q, want %q", i, got, tc.geomType)
		}
		if layers[i].Source != "map" || layers[i].SourceLayer != "mixed" {
			t.Errorf("layer %d source metadata = %q/%q, want map/mixed", i, layers[i].Source, layers[i].SourceLayer)
		}
		if layers[i].Layout == nil || layers[i].Layout.Visibility != style.LayoutVisible {
			t.Errorf("layer %d is not visible", i)
		}
	}
}

func TestUniqueStyleLayerID(t *testing.T) {
	used := map[string]bool{
		"roads":        true,
		"roads-line":   true,
		"roads-line-2": true,
	}
	if got := uniqueStyleLayerID("roads-line", used); got != "roads-line-3" {
		t.Fatalf("uniqueStyleLayerID() = %q, want roads-line-3", got)
	}
}
