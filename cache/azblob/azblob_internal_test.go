package azblob

import "testing"

func TestPadBy512(t *testing.T) {
	type tcase struct {
		x int
		y int32
	}

	fn := func(tc tcase, t *testing.T){
		y := padBy512(tc.x)

		if y != tc.y {
			t.Errorf("incorrect output %d, expected %d", y, tc.y)
		}
	}

	testcases := map[string]tcase{
		"1": {
			x: BlobHeaderLen,
			y: 512,
		},
		"2": {
			x: 512,
			y: 512,
		},
		"3": {
			x: 511,
			y: 512,
		},
		"4": {
			x: 1024,
			y: 1024,
		},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			fn(v, t)
		})
	}
}
