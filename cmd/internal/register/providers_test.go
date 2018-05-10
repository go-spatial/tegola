package register_test

import (
	"testing"

	"github.com/go-spatial/tegola/cmd/internal/register"
)

func TestProviders(t *testing.T) {
	type tcase struct {
		config      []map[string]interface{}
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {
		var err error

		_, err = register.Providers(tc.config)
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

	tests := map[string]tcase{
		"missing name": {
			config: []map[string]interface{}{
				{
					"type": "postgis",
				},
			},
			expectedErr: register.ErrProviderNameMissing,
		},
		"name is not string": {
			config: []map[string]interface{}{
				{
					"name": 1,
				},
			},
			expectedErr: register.ErrProviderNameInvalid,
		},
		"missing type": {
			config: []map[string]interface{}{
				{
					"name": "test",
				},
			},
			expectedErr: register.ErrProviderTypeMissing{"test"},
		},
		"invalid type": {
			config: []map[string]interface{}{
				{
					"name": "test",
					"type": 1,
				},
			},
			expectedErr: register.ErrProviderTypeInvalid{"test"},
		},
		"already registered": {
			config: []map[string]interface{}{
				{
					"name": "test",
					"type": "debug",
				},
				{
					"name": "test",
					"type": "debug",
				},
			},
			expectedErr: register.ErrProviderAlreadyRegistered{"test"},
		},
		"success": {
			config: []map[string]interface{}{
				{
					"name": "test",
					"type": "debug",
				},
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
