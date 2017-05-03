package maths

import (
	"testing"

	"github.com/gdey/tbltest"
)

func TestDouglasPeucker(t *testing.T) {
	type testcase struct {
		line         []Pt
		simplify     bool
		tolerance    float64
		expectedLine []Pt
	}
	tests := tbltest.Cases()
	tests.Run(nil)
}
