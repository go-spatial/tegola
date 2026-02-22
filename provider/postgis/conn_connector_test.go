package postgis

import (
	"testing"
)

func TestConnector(t *testing.T) {
	type tcase struct{}

	fn := func(t *testing.T, tc tcase) {}

	tcases := map[string]tcase{
		"": {},
	}

	for name, tc := range tcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
