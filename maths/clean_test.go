package maths

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/basic"
)

func TestCleanPolygon(t *testing.T) {
	type testcase struct {
		Desc            string
		Polygon         basic.Polygon
		Expected        []basic.Polygon
		ExpectedInvalid basic.Polygon
	}

	tests := tbltest.Cases(
		testcase{
			Desc: "empty Polygon, Should return nothing.",
		},
		testcase{
			Desc: "single Polygon, with bad counter clockwise first line.",
			Polygon: basic.Polygon{
				basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
			},
			ExpectedInvalid: basic.Polygon{
				basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
			},
		},
	)

	tests.Run(t, func(test testcase) {
		poly, invalid := cleanPolygon(test.Polygon)
		if len(test.Expected) != len(poly) {
			t.Errorf("Expected len to get %v got %v", len(test.Expected), len(poly))
		}
	})

}
