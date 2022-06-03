package ttools

import (
	"os"
	"testing"
)

const (
	ENV_TEST_VARIABLE       = "test"
	ENV_TEST_EMPTY_VARIABLE = ""
)

func setEnv() {
	os.Setenv("ENV_TEST_VARIABLE", ENV_TEST_VARIABLE)
	os.Setenv("ENV_TEST_EMPTY_VARIABLE", ENV_TEST_EMPTY_VARIABLE)
}

func unsetEnv() {
	os.Unsetenv("ENV_TEST_VARIABLE")
	os.Unsetenv("ENV_TEST_EMPTY_VARIABLE")
}

func TestGetEnvDefault(t *testing.T) {
	type tcase struct {
		key      string
		dvalue   string
		expected string
	}

	setEnv()
	defer unsetEnv()

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {

			v := GetEnvDefault(tc.key, tc.dvalue)

			if v != tc.expected {
				t.Errorf("\n\nexpected: %s \ngot: %s \n\n", tc.expected, v)
			}
		}
	}

	tests := map[string]tcase{
		"should use default value": {
			key:      "ENV_THAT_DOESNT_EXIST",
			dvalue:   "DEFAULT_VALUE",
			expected: "DEFAULT_VALUE",
		},
		"should get variable from environment": {
			key:      "ENV_TEST_VARIABLE",
			dvalue:   "DEFAULT_VALUE",
			expected: ENV_TEST_VARIABLE,
		},
		"should use default when key is empty string": {
			key:      "",
			dvalue:   "DEFAULT_VALUE",
			expected: "DEFAULT_VALUE",
		},
		"should get variable from empty string environment": {
			key:      "ENV_TEST_EMPTY_VARIABLE",
			dvalue:   "DEFAULT_VALUE",
			expected: ENV_TEST_EMPTY_VARIABLE,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
