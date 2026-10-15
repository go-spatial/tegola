package cache_test

import (
	"runtime"
	"testing"

	"github.com/alexeydott/tegola/cache"
)

// P6-33 regression: Key.String must join key parts with forward slashes on
// every OS. filepath.Join produced backslash-separated keys on Windows, which
// leaked into S3/GCS object keys and cache paths shared across hosts.
func TestKeyStringUsesForwardSlashes(t *testing.T) {
	key := cache.Key{MapName: "map", LayerName: "layer", Z: 0, X: 1, Y: 2}
	if got, want := key.String(), "map/layer/0/1/2"; got != want {
		t.Errorf("Key.String() = %q, want %q", got, want)
	}
}

// Synthetic keys (empty map and layer names) must keep collapsing to the bare
// z/x/y form: the cache/file/testfiles/tegola-cache/0/1/12 fixture and the
// multilevel cache rely on it (P5-17 keeps this behavior).
func TestKeyStringSyntheticKeyCollapses(t *testing.T) {
	key := cache.Key{Z: 0, X: 1, Y: 2}
	if got, want := key.String(), "0/1/2"; got != want {
		t.Errorf("Key.String() = %q, want %q", got, want)
	}
}

// Windows-style key input (backslash separators, as accepted by ParseKey on
// windows hosts) must still construct canonical forward-slash keys (P6-33).
func TestKeyStringWindowsStyleInput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("ParseKey accepts backslash separators on windows hosts only")
	}

	key, err := cache.ParseKey(`map\layer\2\1\2`)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if got, want := key.String(), "map/layer/2/1/2"; got != want {
		t.Errorf("Key.String() = %q, want %q", got, want)
	}
}
