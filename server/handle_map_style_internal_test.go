package server

import "testing"

func TestStringToColorHex(t *testing.T) {
	testcases := []struct {
		input    string
		expected string
	}{
		{
			input:    "alex rolek",
			expected: "#33ce8a",
		},
	}

	for i, tc := range testcases {
		output := stringToColorHex(tc.input)

		if tc.expected != output {
			t.Errorf("testcase (%v) failed. exected (%v) does not match output (%v)", i, tc.expected, output)
		}
	}
}
