package maths_test

import (
	"log"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
)

func Test_Contains(t *testing.T) {
	type TestCase struct {
		subject  []float64
		pt       maths.Pt
		expected bool
		err      error
	}

	subjects := [][]float64{
		{-10, -4, 10, -4, 10, 8, -10, 8, -10, 5, -7, 2, -3, 5, 5, 5, 5, -3, -5, -3, -5, 0, -10, 2},
	}
	newTest := func(idx int, x, y float64, e bool) TestCase {
		return TestCase{
			subject:  subjects[idx],
			pt:       maths.Pt{x, y},
			expected: e,
		}
	}
	tests := tbltest.Cases(
		newTest(0, 0, 0, false),   // 0
		newTest(0, 0, 2, false),   // 1
		newTest(0, 0, 6, true),    // 2
		newTest(0, 7, 0, true),    // 3
		newTest(0, 7, 2, true),    // 4
		newTest(0, 15, 2, false),  // 5
		newTest(0, -15, 2, false), // 6
	)
	tests.Run(func(idx int, tc TestCase) {
		log.Println("Starting Test", idx)
		got, err := maths.Contains(tc.subject, tc.pt)
		log.Printf("Test (%v) Got: %v, err: %v", idx, got, err)
		if err != tc.err {
			t.Errorf("Test (%v) Failed Error Got: %v, wanted %v", idx, err, tc.err)
		}
		if got != tc.expected {
			t.Errorf("Test (%v) Failed Got: %v, wanted %v", idx, got, tc.expected)
		}
	})

}
