package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

func TestGzipDecompressResponseWriter(t *testing.T) {
	type tcase struct {
		data         []byte
		responseCode int
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			var err error
			var buf bytes.Buffer

			if tc.responseCode < 400 {
				// create a new gzip writer to compress our data
				gzipWriter := gzip.NewWriter(&buf)

				// compress the data
				_, err = gzipWriter.Write(tc.data)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}

				// close and flush the writer
				if err = gzipWriter.Close(); err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
			} else {
				_, err := buf.Write(tc.data)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
			}

			// capture our mock response
			recorder := httptest.NewRecorder()
			// wrap our recorder in our response writer
			w := gzipDecompressResponseWriter{
				resp: recorder,
			}

			w.WriteHeader(tc.responseCode)
			// write to our response writer. this should decompress the gzipped data
			_, err = w.Write(buf.Bytes())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// the buffered response is decompressed and flushed when the
			// handler has finished
			w.finish()

			//	0 len compare is not caught by reflect.DeepEqual
			if len(recorder.Body.Bytes()) == 0 && len(tc.data) == 0 {
				return
			}

			// validate our output matches our initial input (pre gzip)
			if !reflect.DeepEqual(recorder.Body.Bytes(), tc.data) {
				t.Errorf("expected (%v) got (%v)", tc.data, recorder.Body.Bytes())
				return
			}

			// decompressed bodies are sent with an accurate Content-Length
			if tc.responseCode < 400 {
				want := strconv.Itoa(len(tc.data))
				if got := recorder.Header().Get("Content-Length"); got != want {
					t.Errorf("Content-Length = %q, want %q", got, want)
				}
			}
		}
	}

	tests := map[string]tcase{
		"decompress": {
			responseCode: http.StatusOK,
			data:         []byte("tegola"),
		},
		"internal server error": {
			responseCode: http.StatusInternalServerError,
			data:         []byte("tegola"),
		},
		"no data": {
			responseCode: http.StatusOK,
			data:         []byte(""),
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestGzipDecompressResponseWriterMultiWrite(t *testing.T) {
	const payload = "a gzip stream written in multiple chunks decodes correctly"

	// compress the payload
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write([]byte(payload)); err != nil {
		t.Fatalf("compressing: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	w := gzipDecompressResponseWriter{resp: recorder}

	w.WriteHeader(http.StatusOK)

	// write the compressed stream in several chunks: the decoder must see the
	// stream as a whole instead of failing on each chunk boundary
	stream := compressed.Bytes()
	chunks := [][]byte{
		stream[:2],
		stream[2 : len(stream)/2],
		stream[len(stream)/2 : len(stream)-1],
		stream[len(stream)-1:],
	}
	for i, chunk := range chunks {
		n, err := w.Write(chunk)
		if err != nil {
			t.Fatalf("write chunk %d: %v", i, err)
		}
		if n != len(chunk) {
			t.Fatalf("write chunk %d returned %d bytes, want %d", i, n, len(chunk))
		}
	}

	w.finish()

	if got := recorder.Body.String(); got != payload {
		t.Fatalf("decompressed body = %q, want %q", got, payload)
	}
	wantLen := strconv.Itoa(len(payload))
	if got := recorder.Header().Get("Content-Length"); got != wantLen {
		t.Fatalf("Content-Length = %q, want %q", got, wantLen)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
}

func TestGzipDecompressResponseWriterWriteReturnsUncompressedByteCount(t *testing.T) {
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write([]byte("tegola")); err != nil {
		t.Fatalf("compressing: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	w := gzipDecompressResponseWriter{resp: recorder}

	stream := compressed.Bytes()
	n, err := w.Write(stream)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(stream) {
		t.Fatalf("Write returned %d, want %d (bytes accepted for buffering)", n, len(stream))
	}

	w.finish()
}

func TestGzipDecompressResponseWriterInvalidGzip(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := gzipDecompressResponseWriter{resp: recorder}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("this is not a gzip stream")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// a body that cannot be decompressed is a broken response: the client
	// receives an explicit error instead of corrupted bytes
	w.finish()

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}
