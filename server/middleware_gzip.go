package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/internal/log"
)

// GZipHandler is responsible for determining if the incoming request should be served gzipped data.
// All response data is assumed to be compressed prior to being passed to this handler.
//
// If the incoming request accepts gzip, successful responses with a body are
// returned with the "Content-Encoding: gzip" header. Error and no-content
// responses are returned without a content encoding.
//
// If no "Accept-Encoding" header is present, or gzip is absent/disabled by
// its quality value, the response is decompressed prior to being sent to the
// client.
func GZipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")

		acceptEncoding := r.Header.Get("Accept-Encoding")
		if acceptsGzip(acceptEncoding) {
			next.ServeHTTP(&gzipResponseWriter{resp: w}, r)
			return
		}

		decompress := &gzipDecompressResponseWriter{resp: w}
		next.ServeHTTP(decompress, r)
		decompress.finish()
	})
}

func acceptsGzip(header string) bool {
	var gzipQuality float64
	var wildcardQuality float64
	var hasGzip, hasWildcard bool

	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		if coding == "" {
			continue
		}

		quality, valid := encodingQuality(parts[1:])
		if !valid {
			continue
		}

		switch coding {
		case "gzip":
			gzipQuality = quality
			hasGzip = true
		case "*":
			wildcardQuality = quality
			hasWildcard = true
		}
	}

	// An explicit gzip entry is more specific than a wildcard entry, even
	// when the explicit entry disables gzip.
	if hasGzip {
		return gzipQuality > 0
	}
	return hasWildcard && wildcardQuality > 0
}

func encodingQuality(parameters []string) (float64, bool) {
	quality := 1.0
	for _, parameter := range parameters {
		key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}

		parsed, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(value), `"`), 64)
		if err != nil || parsed < 0 || parsed > 1 {
			return 0, false
		}
		quality = parsed
		break
	}
	return quality, true
}

// gzipResponseWriter delays setting Content-Encoding until the downstream
// handler has selected a successful response that can contain a body.
type gzipResponseWriter struct {
	status int
	resp   http.ResponseWriter
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.resp.Header()
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.resp.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
	if statusCanHaveBody(status) {
		w.resp.Header().Set("Content-Encoding", "gzip")
	} else {
		w.resp.Header().Del("Content-Encoding")
		if status == http.StatusNoContent || status == http.StatusResetContent {
			w.resp.Header().Del("Content-Length")
		}
	}
	w.resp.WriteHeader(status)
}

// gzipDecompressResponseWriter is responsible for decompressing successful
// responses that contain the pre-compressed tile body. The compressed body is
// buffered and decompressed in one piece once the handler has finished: a gzip
// stream may be written with several Write calls, and decompressing per-Write
// only works for single-write bodies. The decompressed body is sent with an
// accurate Content-Length.
type gzipDecompressResponseWriter struct {
	status    int
	committed bool
	resp      http.ResponseWriter
	buf       bytes.Buffer
}

func (w *gzipDecompressResponseWriter) Header() http.Header {
	return w.resp.Header()
}

func (w *gzipDecompressResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}

	//	check that we have an OK response, if not, don't process the body
	if !statusCanHaveBody(w.status) {
		if !w.committed {
			w.resp.WriteHeader(w.status)
			w.committed = true
		}
		return w.resp.Write(b)
	}

	//	buffer the compressed bytes; the gzip stream can span multiple writes
	//	and is decompressed as a whole in finish()
	return w.buf.Write(b)
}

func (w *gzipDecompressResponseWriter) WriteHeader(i int) {
	if w.status != 0 {
		return
	}
	w.resp.Header().Del("Content-Length")
	w.resp.Header().Del("Content-Encoding")
	w.status = i
}

// finish flushes the decompressed response. It must be called after the
// wrapped handler has returned, before any other handler writes to the
// underlying ResponseWriter.
func (w *gzipDecompressResponseWriter) finish() {
	if w.status == 0 || w.committed {
		return
	}

	if !statusCanHaveBody(w.status) {
		w.resp.WriteHeader(w.status)
		w.committed = true
		return
	}

	body, err := decompressGzip(w.buf.Bytes())
	if err != nil {
		// per the middleware contract all body-carrying responses are
		// pre-compressed by the handler; anything else is a broken response
		log.Errorf("gzip middleware: error decompressing response: %v", err)
		http.Error(w.resp, "error decompressing response", http.StatusInternalServerError)
		w.committed = true
		return
	}

	w.resp.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.resp.WriteHeader(w.status)
	w.committed = true
	if len(body) > 0 {
		_, _ = w.resp.Write(body)
	}
}

// decompressGzip decodes a complete gzip stream. An empty input is treated as
// an empty body.
func decompressGzip(compressed []byte) ([]byte, error) {
	if len(compressed) == 0 {
		return nil, nil
	}

	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	return io.ReadAll(r)
}

func statusCanHaveBody(status int) bool {
	return status >= http.StatusOK &&
		status < http.StatusMultipleChoices &&
		status != http.StatusNoContent &&
		status != http.StatusResetContent
}
