package env_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/go-test/deep"

	"github.com/go-spatial/tegola/internal/env"
)

func TestParseURL(t *testing.T) {
	type tcase struct {
		in          any
		expected    *url.URL
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			got, err := env.ParseURL(tc.in)
			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected err %v, got nil", tc.expectedErr.Error())
					return
				}

				// compare error messages
				if errors.Is(tc.expectedErr, err) {
					t.Errorf("invalid error. expected %v, got %v", tc.expectedErr, err)
					return
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if diff := deep.Equal(got, tc.expected); diff != nil {
				t.Fatalf("expected does not match go: %v", diff)
			}
		}
	}

	tests := map[string]tcase{
		"happy path": {
			in: "https://go-spatial.org/tegola",
			expected: &url.URL{
				Scheme: "https",
				Host:   "go-spatial.org",
				Path:   "/tegola",
			},
		},
		"invalid url escape": {
			in:          "https://go-spatial.org/tegola/_20_%+off_60000_",
			expectedErr: url.EscapeError(""),
		},
		"nil": {
			in:       nil,
			expected: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
