package server

import (
	"fmt"
	"testing"
)

func TestStringToColorHex(t *testing.T) {
	type tcase struct {
		input    string
		expected string
	}
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			output := stringToColorHex(tc.input)

			if tc.expected != output {
				t.Errorf("color hex. expected (%v) got (%v)", tc.expected, output)
			}
		}
	}
	testcases := []tcase{
		{
			input:    "alex rolek",
			expected: "#33ce8a",
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("%d %v", i, tc.input), fn(tc))
	}
}
