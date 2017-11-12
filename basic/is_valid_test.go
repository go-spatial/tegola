package basic_test

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/basic"
)

func TestIsValidLine(t *testing.T) {
	type testcase struct {
		desc     string
		line     basic.Line
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			desc:     "Invalid Line with point duplicated.",
			line:     basic.NewLine(0, 0, 1, 1, 1, 1, 2, 2),
			expected: false,
		},
		testcase{
			desc:     "Invalid Line with an intersection point.",
			line:     basic.NewLine(1, 1, 3, 1, 5, 3, 6, 5, 7, 7, 9, 7, 10, 6, 10, 5, 3, 9, 2, 8, 2, 6, 3, 3, 2, 4),
			expected: false,
		},
		testcase{ // 2
			desc:     "Valid line.",
			line:     basic.NewLine(1, 1, 3, 1, 5, 3, 6, 5, 7, 7, 9, 7, 10, 6, 10, 5, 11, 5, 2, 18),
			expected: true,
		},
		testcase{ // 3
			desc:     "Valid line2",
			line:     basic.NewLine(4, 2, 6, 2, 8, 3, 8, 5, 7, 7, 5, 8, 3, 7, 2, 6, 2, 4),
			expected: true,
		},
	)
	//tests.RunOrder = "2"
	tests.Run(func(idx int, test testcase) {
		got := test.line.IsValid()
		if got != test.expected {
			t.Errorf("Test %v, expected %v got %v", idx, test.expected, got)
		}
	})
}

func TestIsValidPolygon(t *testing.T) {
	type testcase struct {
		desc     string
		polygon  basic.Polygon
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			desc: "Standard one line Polygon",
			polygon: basic.Polygon{
				basic.NewLine(4, 2, 6, 2, 8, 3, 8, 5, 7, 7, 5, 8, 3, 7, 2, 6, 2, 4),
			},
			expected: true,
		},
	)
	tests.Run(func(idx int, test testcase) {
		got := test.polygon.IsValid()
		if got != test.expected {
			t.Errorf("Test %v, expected %v got %v", idx, test.expected, got)
		}
	})
}
