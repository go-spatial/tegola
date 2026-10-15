package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// minimal TOML accepted by Parse: a single map entry
const loadTestTOML = "[[maps]]\nname = \"test\"\n"

// TestLoadHTTPStatus ensures Load reports non-2xx HTTP responses as errors
// (including the status in the message) instead of feeding an error page to
// the TOML parser (part13 P6-27).
func TestLoadHTTPStatus(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("<html><body>not a config</body></html>"))
		}))

		_, err := Load(srv.URL)
		srv.Close()

		if err == nil {
			t.Errorf("expected Load to fail for HTTP status %d, got nil", status)
			continue
		}
		if !strings.Contains(err.Error(), strconv.Itoa(status)) {
			t.Errorf("expected error to mention the HTTP status %d, got: %v", status, err)
		}
	}
}

// TestLoadHTTPSuccess ensures a 2xx response carrying valid TOML parses.
func TestLoadHTTPSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(loadTestTOML))
	}))
	defer srv.Close()

	conf, err := Load(srv.URL)
	if err != nil {
		t.Fatalf("expected Load to succeed, got: %v", err)
	}
	if len(conf.Maps) != 1 || conf.Maps[0].Name != "test" {
		t.Errorf("unexpected config parsed: %+v", conf)
	}
}

// TestLoadLocalFileClosed ensures the local file branch closes the file it
// opened: on Windows an open handle makes os.Remove fail (part13 P6-27).
func TestLoadLocalFileClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(loadTestTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	conf, err := Load(path)
	if err != nil {
		t.Fatalf("expected Load to succeed, got: %v", err)
	}
	if len(conf.Maps) != 1 || conf.Maps[0].Name != "test" {
		t.Errorf("unexpected config parsed: %+v", conf)
	}

	// succeeds only if Load closed its file handle
	if err := os.Remove(path); err != nil {
		t.Errorf("config file still locked after Load (file handle leaked?): %v", err)
	}
}
