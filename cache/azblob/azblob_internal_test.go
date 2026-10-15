package azblob

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/dict"
)

func TestValidateContainerURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{name: "azure blob endpoint", value: "https://account.blob.core.windows.net/container", ok: true},
		{name: "sas query", value: "https://account.blob.core.windows.net/container?sv=2019-02-02&sig=abc", ok: true},
		{name: "azurite path", value: "http://127.0.0.1:10000/devstoreaccount1/test", ok: true},
		{name: "empty", value: ""},
		{name: "not a url", value: "not a url"},
		{name: "wrong scheme", value: "ftp://account.blob.core.windows.net/container"},
		{name: "missing host", value: "https:///container"},
		{name: "missing path", value: "https://account.blob.core.windows.net"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateContainerURL(tc.value)
			if tc.ok && err != nil {
				t.Errorf("expected %q to be valid, got %v", tc.value, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("expected %q to be rejected", tc.value)
			}
		})
	}
}

func TestNewRejectsInvalidContainerURL(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "not a url", "https://", "https://account.blob.core.windows.net"} {
		_, err := New(dict.Dict{ConfigKeyContainerUrl: value})
		if err == nil {
			t.Errorf("expected New(%q) to fail", value)
		}
	}
}

// TestPurgeNotFoundIsSuccess exercises the real client against an in-memory
// blob service: deleting an already-absent blob (HTTP 404) must count as
// success, matching the file cache semantics (idempotent Purge).
func TestPurgeNotFoundIsSuccess(t *testing.T) {
	var (
		mu    sync.Mutex
		blobs = make(map[string][]byte)
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			blobs[r.URL.Path] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			val, ok := blobs[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(val)
		case http.MethodDelete:
			if _, ok := blobs[r.URL.Path]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(blobs, r.URL.Path)
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	c, err := New(dict.Dict{
		ConfigKeyContainerUrl:     fmt.Sprintf("%s/devstoreaccount1/testcontainer", srv.URL),
		ConfigKeyAzureAccountName: "devstoreaccount1",
		ConfigKeyAzureSharedKey:   base64.StdEncoding.EncodeToString([]byte("fake-test-key-0123456789abcdef")),
	})
	if err != nil {
		t.Fatalf("New() failed against stub service: %v", err)
	}

	key := cache.Key{MapName: "test-map", LayerName: "test-layer", Z: 4, X: 8, Y: 7}
	if err := c.Set(context.Background(), &key, []byte("payload")); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// First Purge removes the blob; the second must tolerate the 404.
	if err := c.Purge(context.Background(), &key); err != nil {
		t.Fatalf("first Purge() failed: %v", err)
	}
	if err := c.Purge(context.Background(), &key); err != nil {
		t.Errorf("Purge() of a missing blob must succeed, got: %v", err)
	}
}

// P6-33-class regression: blob keys must join key parts with forward slashes
// on every OS. filepath.Join produced backslash-separated blob keys on
// Windows, so blobs written on Windows never matched reads on other hosts.
func TestBlobKeyUsesForwardSlashes(t *testing.T) {
	azb := Cache{Basepath: "mybase"}
	key := cache.Key{MapName: "map", LayerName: "layer", Z: 1, X: 2, Y: 3}
	if got, want := azb.blobKey(&key), "mybase/map/layer/1/2/3"; got != want {
		t.Errorf("blobKey() = %q, want %q", got, want)
	}
}
