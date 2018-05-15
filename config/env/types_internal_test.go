package env

import (
	"os"
	"testing"
)

func TestReplaceEnvVar(t *testing.T) {
	type tcase struct {
		envVar      map[string]string
		in          string
		expected    string
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		// set the environment vars
		for i := range tc.envVar {
			os.Setenv(i, tc.envVar[i])
		}

		// issue replace
		out, err := replaceEnvVar(tc.in)
		if err != nil {
			if tc.expectedErr.Error() != err.Error() {
				t.Errorf("expected %v, got %v", tc.expected, err)
				return
			}
		}

		if tc.expected != out {
			t.Errorf("expected %v, got %v", tc.expected, out)
			return
		}
	}

	tests := map[string]tcase{
		"success": {
			envVar: map[string]string{
				"FOO": "bar",
			},
			in:       "${FOO}",
			expected: "bar",
		},
		"env var not found": {
			envVar:      map[string]string{},
			in:          "${BAZ}",
			expectedErr: EnvironmentError{"BAZ"},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
