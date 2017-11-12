package maths

import (
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola"
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
		testcase{
			Desc: "A single polygon with a bad initial line, and then a good line.",
			Polygon: basic.Polygon{
				basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
				basic.NewLine(1, 1, 9, 1, 9, 9, 1, 9),
				basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
			},
			Expected: []basic.Polygon{
				basic.Polygon{
					basic.NewLine(1, 1, 9, 1, 9, 9, 1, 9),
					basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
				},
			},
			ExpectedInvalid: basic.Polygon{
				basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		poly, _ := cleanPolygon(test.Polygon)
		if len(test.Expected) != len(poly) {
			t.Errorf("Test %v: Expected len to get %v got %v", idx, len(test.Expected), len(poly))
		}
	})
}

func TestCleanMultiPolygon(t *testing.T) {
	type testcase struct {
		Desc         string
		MultiPolygon basic.MultiPolygon
		Expected     basic.MultiPolygon
		ExpectedErr  error
	}
	tests := tbltest.Cases(
		testcase{
			Desc: "Empty MultiPolygon",
		},
		testcase{
			Desc: "MultiPolygon with a polygon broken up.",
			MultiPolygon: basic.MultiPolygon{
				basic.Polygon{
					basic.NewLine(1, 1, 9, 1, 9, 9, 1, 9),
				},
				basic.Polygon{
					basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
				},
			},
			Expected: basic.MultiPolygon{
				basic.Polygon{
					basic.NewLine(1, 1, 9, 1, 9, 9, 1, 9),
					basic.NewLine(4, 2, 2, 4, 2, 6, 3, 7, 5, 8, 7, 7, 8, 5, 8, 3, 6, 2),
				},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		got, gotErr := cleanMultiPolygon(test.MultiPolygon)
		if gotErr != test.ExpectedErr {
			t.Errorf("Test %v: Expected error %v, got %v", idx, test.ExpectedErr, gotErr)
		}
		if test.ExpectedErr == nil && !tegola.IsMultiPolygonEqual(got, test.Expected) {
			t.Errorf("Test %v: Expected %#v, got %#v", idx, test.Expected, got)
		}
	})

}
