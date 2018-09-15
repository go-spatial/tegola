package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// GZipHandler is responsible for determining if the incoming request should be served gzipped data.
// All response data is assumed to be compressed prior to being passed to this handler.
//
// If the incoming request has the "Accept-Encoding" header set with the values of "gzip" or "*"
// the response header "Content-Encoding: gzip" is set and the compressed data is returned.
//
// If no "Accept-Encoding" header is present or "Accept-Encoding" has a value of "gzip;q=0" or
// "*;q=0" the response is decompressed prior to being sent to the client.
func GZipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		acceptEncoding := r.Header.Get("Accept-Encoding")
		if acceptEncoding == "" {
			// decompress
			next.ServeHTTP(&gzipDecompressResponseWriter{resp: w}, r)
			return
		}

		decompress := false
		for _, v := range strings.Split(acceptEncoding, ",") {
			if (strings.Contains(v, "gzip") || strings.Contains(v, "*")) && strings.HasSuffix(v, ";q=0") {
				decompress = true
			}
		}

		if decompress {
			next.ServeHTTP(&gzipDecompressResponseWriter{resp: w}, r)
			return
		}

		// set appropriate header
		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(w, r)
		return
	})
}

// gzipDecompressResponseWriter is responsible for decompressing responses
// when the http status code == 200.
type gzipDecompressResponseWriter struct {
	status int
	resp   http.ResponseWriter
}

func (w *gzipDecompressResponseWriter) Header() http.Header {
	return w.resp.Header()
}

func (w *gzipDecompressResponseWriter) Write(b []byte) (int, error) {
	//	check that we have an OK response, if not, don't process the body
	if w.status != http.StatusOK {
		return w.resp.Write(b)
	}

	//	setup new gzip reader
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	defer r.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		return 0, err
	}

	return w.resp.Write(buf.Bytes())
}

func (w *gzipDecompressResponseWriter) WriteHeader(i int) {
	w.status = i
	w.resp.WriteHeader(i)
}
