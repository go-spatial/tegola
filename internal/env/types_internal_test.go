package env

import (
	"testing"
)

func TestReplaceEnvVar(t *testing.T) {
	type tcase struct {
		envVars     map[string]string
		in          string
		expected    string
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			// setup our env vars
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			out, err := replaceEnvVar(tc.in)
			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected err %v, got nil", tc.expectedErr.Error())
					return
				}

				// compare error messages
				if tc.expectedErr.Error() != err.Error() {
					t.Errorf("invalid error. expected %v, got %v", tc.expectedErr, err)
					return
				}

				return
			}
			if err != nil {
				t.Errorf("unexpected err: %v", err)
				return
			}

			if out != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, out)
				return
			}
		}
	}

	tests := map[string]tcase{
		"env": {
			envVars: map[string]string{
				"TEST_STRING": "foo",
			},
			in:       "${TEST_STRING}",
			expected: "foo",
		},
		"env missing": {
			in:          "${TEST_STRING}",
			expectedErr: ErrEnvVar("TEST_STRING"),
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
