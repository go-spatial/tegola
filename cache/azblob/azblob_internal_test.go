package azblob

import "testing"

func TestPadBy512(t *testing.T) {
	type tcase struct {
		n        int
		expected int32
	}

	fn := func(tc tcase, t *testing.T) {
		expected := padBy512(tc.n)

		if expected != tc.expected {
			t.Errorf("got %d, expected %d", expected, tc.expected)
		}
	}

	testcases := map[string]tcase{
		"1": {
			n:        BlobHeaderLen,
			expected: 512,
		},
		"2": {
			n:        512,
			expected: 512,
		},
		"3": {
			n:        511,
			expected: 512,
		},
		"4": {
			n:        1024,
			expected: 1024,
		},
		"5": {
			n:        0,
			expected: 512,
		},
		"6": {
			n:        513,
			expected: 1024,
		},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			fn(v, t)
		})
	}
}
