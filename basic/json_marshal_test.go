package basic

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
)

func TestJSONMarshal(t *testing.T) {
	type testcase struct {
		Geometry Geometry
		JSON     string
		Desc     string
	}
	tests := tbltest.Cases(
		testcase{
			Geometry: Point{10, 10},
			JSON:     `{"type":"Point","coordinates":[10,10]}`,
			Desc:     `Test Point.`,
		},
		testcase{
			Geometry: MultiPoint{Point{10, 10}, Point{20, 20}},
			JSON:     `{"type":"MultiPoint","coordinates":[[10,10],[20,20]]}`,
			Desc:     `Test MultiPoint.`,
		},
		testcase{
			Geometry: Point3{10, 10, 10},
			JSON:     `{"type":"Point3","coordinates":[10,10,10]}`,
			Desc:     `Test Point3.`,
		},
		testcase{
			Geometry: MultiPoint3{Point3{10, 10, 10}, Point3{20, 20, 20}},
			JSON:     `{"type":"MultiPoint3","coordinates":[[10,10,10],[20,20,20]]}`,
			Desc:     `Test MultiPoint.`,
		},
		testcase{
			Geometry: NewLine(10, 10, 20, 10, 20, 20),
			JSON:     `{"type":"LineString","coordinates":[[10,10],[20,10],[20,20]]}`,
			Desc:     `Test Line.`,
		},
		testcase{
			Geometry: MultiLine{NewLine(10, 10, 20, 10, 20, 20), NewLine(10, 5, 20, 10, 15, 20)},
			JSON:     `{"type":"MultiLineString","coordinates":[[[10,10],[20,10],[20,20]],[[10,5],[20,10],[15,20]]]}`,
			Desc:     `Test MultiLine.`,
		},
		testcase{
			Geometry: Polygon{NewLine(10, 10, 20, 10, 20, 20, 10, 20)},
			JSON:     `{"type":"Polygon","coordinates":[[[10,10],[20,10],[20,20],[10,20]]]}`,
			Desc:     `Test Polygon.`,
		},
		testcase{
			Geometry: MultiPolygon{
				Polygon{NewLine(10, 10, 20, 10, 20, 20, 10, 20)},
				Polygon{NewLine(10, 10, 20, 10, 20, 20), NewLine(10, 5, 20, 10, 15, 20)},
			},
			JSON: `{"type":"MultiPolygon","coordinates":[[[[10,10],[20,10],[20,20],[10,20]]],[[[10,10],[20,10],[20,20]],[[10,5],[20,10],[15,20]]]]}`,
			Desc: `Test MultiPolygon.`,
		},
	)
	tests.Run(func(tc testcase) {
		got, err := json.Marshal(tc.Geometry)
		if err != nil {
			t.Fatalf("Got unexpected error: %v for %v", err, tc.Desc)
		}
		if string(got) != tc.JSON {
			t.Errorf("%v failed, Got: %v want: %v", tc.Desc, string(got), tc.JSON)
		}
	})
	tests.Run(func(tc testcase) {
		geo, err := UnmarshalJSON([]byte(tc.JSON))
		if err != nil {
			t.Fatalf("Got unexpected error: %v for %v", err, tc.Desc)
		}
		if !reflect.DeepEqual(geo, tc.Geometry) {
			t.Errorf("%v failed, Got: %v want: %v", tc.Desc, geo, tc.Geometry)
		}
	})
}
