package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAcceptsGzipHonorsQualityValues(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{header: "", want: false},
		{header: "gzip", want: true},
		{header: "GZip ; q = 0.0", want: false},
		{header: "br, gzip;q=0.5", want: true},
		{header: "br", want: false},
		{header: "*", want: true},
		{header: "*;q=0", want: false},
		{header: "*;q=1, gzip;q=0", want: false},
		{header: "gzip;q=0, *;q=1", want: false},
		{header: "gzip;q=bogus, *;q=1", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.header, func(t *testing.T) {
			if got := acceptsGzip(tc.header); got != tc.want {
				t.Fatalf("acceptsGzip(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

func TestGZipHandlerDecompressesImplicitSuccessfulWrite(t *testing.T) {
	body := []byte("pre-compressed tile")
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	handler := GZipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(compressed.Bytes())
	}))
	request := httptest.NewRequest(http.MethodGet, "/tile", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	if got := recorder.Body.Bytes(); !bytes.Equal(got, body) {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestGZipHandlerLeavesCompressedBodyWhenAccepted(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = io.WriteString(gz, "tile")
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	handler := GZipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(compressed.Bytes())
	}))
	request := httptest.NewRequest(http.MethodGet, "/tile", nil)
	request.Header.Set("Accept-Encoding", "br, gzip ; q = 1.0")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if !bytes.Equal(recorder.Body.Bytes(), compressed.Bytes()) {
		t.Fatal("accepted gzip response body was modified")
	}
}
