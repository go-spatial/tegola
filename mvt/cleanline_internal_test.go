package mvt

import (
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
)

func TestCleanline(t *testing.T) {
	type tcase struct {
		line     basic.Line
		expected basic.Line
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			el := cleanLine(tc.line)

			if !tegola.IsLineStringEqual(el, tc.expected) {
				t.Errorf("expected %v, got %v", tc.line.GoString(), el.GoString())
			}
		}
	}

	tests := map[string]tcase{
		"1": tcase{
			line: basic.Line{ // basic.Line len(000007) direction(clockwise).
				{2046.000000, 1386.000000}, {2047.000000, 1386.000000}, {2047.000000, 1387.000000}, {2046.000000, 1387.000000}, {2046.000000, 1386.000000}, {2046.000000, 1387.000000}, {2046.000000, 1386.000000}, // 000000 — 000006
			},
			expected: basic.Line{
				{2047.000000, 1386.000000}, {2047.000000, 1387.000000}, {2046.000000, 1387.000000}, {2046.000000, 1386.000000},
			},
		},
		"2": tcase{
			basic.Line{ // basic.Line len(000005) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000}, {3651.000000, 1341.000000}, {3651.000000, 1342.000000}, // 000000 — 000004
			},
			basic.Line{ // basic.Line len(000005) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000},
			},
		},
		"3": tcase{
			basic.Line{ // basic.Line len(000010) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3652.000000, 1343.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000}, {3651.000000, 1341.000000}, {3651.000000, 1342.000000}, {3651.000000, 1341.000000}, {3651.000000, 1342.000000}, {3650.000000, 1342.000000}, // 000000 — 000009
			},
			basic.Line{ // basic.Line len(000005) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
