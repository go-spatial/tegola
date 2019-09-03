package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTileCacheResponseWriter(t *testing.T) {
	type tcase struct {
		data         []byte
		responseCode int
		expected     []byte
	}

	tests := map[string]tcase{
		"1": {
			data:         []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
			responseCode: http.StatusOK,
			expected:     []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
		},
		"2": {
			data:         []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
			responseCode: http.StatusInternalServerError,
			expected:     []byte{},
		},
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var buff bytes.Buffer

			rw := newTileCacheResponseWriter(httptest.NewRecorder(), &buff)
			rw.WriteHeader(tc.responseCode)
			_, err := rw.Write(tc.data)
			if err != nil {
				t.Errorf("unable to write to response writer: %v", err)
				return
			}

			if buff.Len() != len(tc.expected) {
				t.Errorf("expected (%v) does not match output (%v)", tc.expected, buff.Bytes())
				return
			}

			if len(tc.expected) > 0 && !reflect.DeepEqual(buff.Bytes(), tc.expected) {
				t.Errorf("expected (%v) does not match output (%v)", tc.expected, buff.Bytes())
				return
			}
		}
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
