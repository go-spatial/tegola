package mvt_test

import (
	"testing"

	"github.com/terrando/tegola/mvt"
)

func TestEncodeCommandInt(t *testing.T) {
	testcases := []struct {
		id       uint32
		count    uint32
		expected uint32
	}{
		{
			id:       mvt.CommandMoveTo,
			count:    uint32(1),
			expected: uint32(9),
		},
		{
			id:       mvt.CommandMoveTo,
			count:    uint32(120),
			expected: uint32(961),
		},
		{
			id:       mvt.CommandLineTo,
			count:    uint32(1),
			expected: uint32(10),
		},
		{
			id:       mvt.CommandLineTo,
			count:    uint32(3),
			expected: uint32(26),
		},
		{
			id:       mvt.CommandClosePath,
			count:    uint32(1),
			expected: uint32(15),
		},
	}

	for i, test := range testcases {
		result := mvt.EncodeCommandInt(test.id, test.count)
		if result != test.expected {
			t.Errorf("Failed Test %v: Expected %v, Got %v\n", i, test.expected, result)
		}
	}

}
