package mvt

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

func TestCleanline(t *testing.T) {
	type testcase struct {
		line     basic.Line
		expected basic.Line
	}
	tests := tbltest.Cases(
		testcase{
			line: basic.Line{ // basic.Line len(000007) direction(clockwise).
				{2046.000000, 1386.000000}, {2047.000000, 1386.000000}, {2047.000000, 1387.000000}, {2046.000000, 1387.000000}, {2046.000000, 1386.000000}, {2046.000000, 1387.000000}, {2046.000000, 1386.000000}, // 000000 — 000006
			},
			expected: basic.Line{
				{2047.000000, 1386.000000}, {2047.000000, 1387.000000}, {2046.000000, 1387.000000}, {2046.000000, 1386.000000},
			},
		},
		testcase{
			basic.Line{ // basic.Line len(000005) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000}, {3651.000000, 1341.000000}, {3651.000000, 1342.000000}, // 000000 — 000004
			},
			basic.Line{ // basic.Line len(000005) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000},
			},
		},
		testcase{
			basic.Line{ // basic.Line len(000010) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3652.000000, 1343.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000}, {3651.000000, 1341.000000}, {3651.000000, 1342.000000}, {3651.000000, 1341.000000}, {3651.000000, 1342.000000}, {3650.000000, 1342.000000}, // 000000 — 000009
			},
			basic.Line{ // basic.Line len(000005) direction(counter clockwise).
				{3650.000000, 1342.000000}, {3651.000000, 1343.000000}, {3651.000000, 1342.000000},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		el := cleanLine(test.line)
		if !tegola.IsLineStringEqual(el, test.expected) {
			t.Errorf("Test %v: Did not get expected line. Got %v", idx, el.GoString())
		}
	})

}
