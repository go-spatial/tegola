package env_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-test/deep"
)

func TestURLUnmarshalTOML(t *testing.T) {
	type tcase struct {
		in          any
		expected    env.URL
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			got := env.URL{}
			err := got.UnmarshalTOML(tc.in)
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
			expected: env.URL{
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
			expected: env.URL{},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
