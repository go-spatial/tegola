package validate

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/testhelpers"
)

func TestIsSimple(t *testing.T) {
	type testcase struct {
		tc       []float64
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			tc:       []float64{0, 0, 0, 10, 10, 10, 10, 0},
			expected: true,
		},
		testcase{
			tc:       []float64{0, 0, 10, 10, 0, 10, 5, -5},
			expected: false,
		},
		testcase{
			tc:       []float64{0, 0, 10, 10, 0, 10, 10, 0},
			expected: false,
		},
		testcase{
			tc:       []float64{0, 0, 10, 0, 5, -5, 5, 0, 10, 1, 100, 100, 50, 50},
			expected: false,
		},
		testcase{ // 4
			tc:       []float64{3921, 3879, 3922, 3879, 3921, 3880, 3920, 3880, 0, 0},
			expected: true,
		},
	)
	//tests.RunOrder = "1"
	tests.Run(func(idx int, tc testcase) {
		segs, err := maths.NewSegments(tc.tc)
		if err != nil {
			t.Fatalf("Bad Test(%v), got error: %v", idx, err)
		}
		got := IsSimple(segs)
		if got != tc.expected {
			t.Fatalf("test(%v) Expected: %v got: %v", idx, tc.expected, got)
		}
	})
}
func TestIsSimpleLines(t *testing.T) {
	type testcase struct {
		segs     []maths.Line
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			segs:     testhelpers.LoadLinesFromFile(`testdata/invalid_polygon.txt`),
			expected: false,
		},
	)
	//tests.RunOrder = "1"
	tests.Run(func(idx int, tc testcase) {
		got := IsSimple(tc.segs)
		if got != tc.expected {
			t.Fatalf("test(%v) Expected: %v got: %v", idx, tc.expected, got)
		}
	})
}

func TestDoesIntersect(t *testing.T) {
	type testcase struct {
		tc       [2]maths.Line
		expected bool
	}
	tests := tbltest.Cases(
		testcase{
			tc: [2]maths.Line{
				maths.NewLine(0, 10, 5, -5),
				maths.NewLine(0, 0, 10, 10),
			},
			expected: true,
		},
		testcase{
			tc: [2]maths.Line{
				maths.NewLine(5, 5, 5, 10),
				maths.NewLine(0, 20, 20, 20),
			},
			expected: false,
		},
		testcase{
			tc: [2]maths.Line{
				maths.NewLine(5, 5, 5, 20),
				maths.NewLine(0, 20, 20, 20),
			},
			expected: true,
		},
		testcase{
			tc: [2]maths.Line{
				maths.NewLine(5, 5, 5, 30),
				maths.NewLine(0, 20, 20, 20),
			},
			expected: true,
		},
		testcase{
			tc: [2]maths.Line{
				maths.NewLine(5, 5, 5, 30),
				maths.NewLine(0, 10, 20, 20),
			},
			expected: true,
		},
		testcase{ // 5
			tc: [2]maths.Line{
				maths.NewLine(5, 5, 5, 30),
				maths.NewLine(6, 10, 20, 20),
			},
			expected: false,
		},
		testcase{ // 6
			tc: [2]maths.Line{
				maths.NewLine(3921, 3879, 3922, 3879),
				maths.NewLine(3921, 3880, 3920, 3880),
			},
		},
	)
	tests.Run(func(idx int, tc testcase) {

		/*
			got := DoesIntersect(tc.tc[0], tc.tc[1])
			if got != tc.expected {
				t.Fatalf("test(%v) Expected: %v got: %v", idx, tc.expected, got)
			}
		*/
	})
}
