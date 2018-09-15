package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"reflect"
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
			// write to our response writer. this should decompres the gzipped data
			_, err = w.Write(buf.Bytes())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			//	0 len compare is not caught by reflect.DeepEqual
			if len(recorder.Body.Bytes()) == 0 && len(tc.data) == 0 {
				return
			}

			// validate our output matches our initial input (pre gzip)
			if !reflect.DeepEqual(recorder.Body.Bytes(), tc.data) {
				t.Errorf("expected (%v) got (%v)", tc.data, recorder.Body.Bytes())
				return
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
