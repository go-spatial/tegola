package register_test

import (
	"testing"

	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/dict"
)

func TestCaches(t *testing.T) {
	type tcase struct {
		config      dict.Dict
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			var err error

			_, err = register.Cache(tc.config)
			if tc.expectedErr != nil {
				if err.Error() != tc.expectedErr.Error() {
					t.Errorf("invalid error. expected: %v, got %v", tc.expectedErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected err: %v", err)
				return
			}
		}
	}

	tests := map[string]tcase{
		"missing type": {
			config:      dict.Dict{},
			expectedErr: register.ErrCacheTypeMissing,
		},

		"type is not string": {
			config: dict.Dict{
				"type": 1,
			},
			expectedErr: register.ErrCacheTypeInvalid,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
