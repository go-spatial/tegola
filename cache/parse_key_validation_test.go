package cache_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/alexeydott/tegola/cache"
)

// P5-17 regression: ParseKey must reject empty, "." and ".." map/layer name
// components. Such names escape the cache root when Key.String joins the key
// into a cache path (".." traversal) or silently collapse ("." / "").
func TestParseKeyRejectsPathElementsInNames(t *testing.T) {
	testcases := map[string]string{
		"map traversal":       "/../evil/0/0/0",
		"layer traversal":     "/map/../0/0/0",
		"map current dir":     "/./layer/0/0/0",
		"layer current dir":   "/map/./0/0/0",
		"empty layer":         "/map//0/0/0",
		"map traversal 4pt":   "/../12/11/123",
		"map current dir 4pt": "/./12/11/123",
	}

	for name, input := range testcases {
		t.Run(name, func(t *testing.T) {
			key, err := cache.ParseKey(input)
			if err == nil {
				t.Fatalf("ParseKey(%q) returned key %+v, want an error (P5-17)", input, key)
			}
			if key != nil {
				t.Errorf("ParseKey(%q) returned a key (%+v) alongside the error, want nil", input, key)
			}
			var nameErr cache.ErrInvalidKeyName
			if !errors.As(err, &nameErr) {
				t.Errorf("ParseKey(%q) error = %v (%T), want cache.ErrInvalidKeyName", input, err, err)
			}
		})
	}
}

// Windows-style separators in hostile input must be rejected the same way
// (ParseKey normalizes them to slashes first).
func TestParseKeyRejectsWindowsStyleTraversal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash separators are only normalized to slashes on windows")
	}

	key, err := cache.ParseKey(`..\evil\0\0\0`)
	if err == nil {
		t.Fatalf("ParseKey returned key %+v, want an error (P5-17)", key)
	}
	var nameErr cache.ErrInvalidKeyName
	if !errors.As(err, &nameErr) {
		t.Errorf("ParseKey error = %v (%T), want cache.ErrInvalidKeyName", err, err)
	}
}

// The synthetic (map-less) key form is load-bearing and must keep parsing and
// collapsing exactly as before (P5-17: only parsed names are validated).
func TestParseKeySyntheticKeyStillParses(t *testing.T) {
	key, err := cache.ParseKey("/0/0/0")
	if err != nil {
		t.Fatalf("ParseKey(/0/0/0) error = %v, want nil", err)
	}
	if key.MapName != "" || key.LayerName != "" {
		t.Errorf("synthetic key names = (%q, %q), want empty map and layer", key.MapName, key.LayerName)
	}
	if got := key.String(); got != "0/0/0" {
		t.Errorf("synthetic key.String() = %q, want %q (empty names must still collapse)", got, "0/0/0")
	}
}
