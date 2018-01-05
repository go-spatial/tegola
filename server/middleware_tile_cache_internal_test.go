package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTileCacheResponseWriter(t *testing.T) {
	testcases := []struct {
		data         []byte
		responseCode int
		expected     []byte
	}{
		{
			data:         []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
			responseCode: http.StatusOK,
			expected:     []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
		},
		{
			data:         []byte{0x53, 0x69, 0x6c, 0x61, 0x73},
			responseCode: http.StatusInternalServerError,
			expected:     []byte{},
		},
	}

	for i, tc := range testcases {
		var buff bytes.Buffer

		rw := newTileCacheResponseWriter(httptest.NewRecorder(), &buff)
		rw.WriteHeader(tc.responseCode)
		_, err := rw.Write(tc.data)
		if err != nil {
			t.Errorf("[%v] unable to write to response writer: %v", i, err)
			continue
		}

		if buff.Len() != len(tc.expected) {
			t.Errorf("[%v] expected (%v) does not match output (%v)", i, tc.expected, buff.Bytes())
			continue
		}

		if len(tc.expected) > 0 && !reflect.DeepEqual(buff.Bytes(), tc.expected) {
			t.Errorf("[%v] expected (%v) does not match output (%v)", i, tc.expected, buff.Bytes())
			return
		}
	}
}
