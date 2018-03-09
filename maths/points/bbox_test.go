package points

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola"
)

func TestDisjointBB(t *testing.T) {
	// BoundBox coordinates are in the form [MinX, MinY, MaxX, MaxY]
	bboxCenter := BoundingBox{10, 10, 20, 20}
	type TestCase struct {
		bbox1    BoundingBox
		bbox2    BoundingBox
		disjoint bool
	}
	bboxLeftOfCenter := BoundingBox{0, 10, 9, 20}
	bboxRigthOfCenter := BoundingBox{21, 10, 30, 10}
	bboxAboveCenter := BoundingBox{10, 21, 20, 30}
	bboxBelowCenter := BoundingBox{10, 0, 20, 9}

	bboxOverlapLeft := BoundingBox{0, 10, 11, 20}
	bboxOverlapRight := BoundingBox{19, 10, 30, 20}
	bboxOverlapTop := BoundingBox{10, 19, 20, 30}
	bboxOverlapBottom := BoundingBox{10, 0, 20, 11}

	bboxInsideCenter := BoundingBox{11, 11, 19, 19}

	testCases := []TestCase{
		{
			bbox1:    bboxCenter,
			bbox2:    bboxLeftOfCenter,
			disjoint: true,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxRigthOfCenter,
			disjoint: true,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxAboveCenter,
			disjoint: true,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxBelowCenter,
			disjoint: true,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxOverlapLeft,
			disjoint: false,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxOverlapRight,
			disjoint: false,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxOverlapTop,
			disjoint: false,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxOverlapBottom,
			disjoint: false,
		},
		{
			bbox1:    bboxCenter,
			bbox2:    bboxInsideCenter,
			disjoint: false,
		},
	}

	for i, tc := range testCases {
		disjoint := tc.bbox1.DisjointBB(tc.bbox2)
		if !reflect.DeepEqual(tc.disjoint, disjoint) {
			t.Errorf(" [%v] disjoint, expected %v got %v", i, tc.disjoint, disjoint)
		}
	}
}

func TestConvertSRID(t *testing.T) {
	type TestCase struct {
		bbox         BoundingBox
		fromSRID     uint64
		toSRID       uint64
		expectedBbox BoundingBox
	}

	testCases := [...]TestCase{
		{
			bbox:     BoundingBox{26.0, 32.0, 27.0, 33.0},
			fromSRID: tegola.WebMercator,
			toSRID:   tegola.WGS84,
			expectedBbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
		},
		{
			bbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
			fromSRID:     tegola.WGS84,
			toSRID:       tegola.WebMercator,
			expectedBbox: BoundingBox{26.0, 32.0, 27.0, 33.0},
		},
		{
			bbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
			fromSRID: tegola.WebMercator,
			toSRID:   tegola.WebMercator,
			expectedBbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
		},
		{
			bbox:         BoundingBox{26.0, 32.0, 27.0, 33.0},
			fromSRID:     tegola.WGS84,
			toSRID:       tegola.WGS84,
			expectedBbox: BoundingBox{26.0, 32.0, 27.0, 33.0},
		},
	}

	floatDelta := 0.00000001
	for i, tc := range testCases {
		convertedBBox := tc.bbox.ConvertSRID(tc.fromSRID, tc.toSRID)
		failMsg := fmt.Sprintf("TestCase[%v]: %v != %v", i, convertedBBox, tc.expectedBbox)
		for j := 0; j < 4; j++ {
			if math.Abs(tc.expectedBbox[j]-convertedBBox[j]) > floatDelta {
				t.Error(failMsg)
			}
		}
	}
}

func TestAsGeoJSON(t *testing.T) {
	type TestCase struct {
		bbox            BoundingBox
		expectedGeoJson string
	}

	testCases := []TestCase{
		{
			bbox: BoundingBox{0.1234, 0.1234, 0.2345, 0.2345},
			expectedGeoJson: `
{
  "type": "Polygon",
  "coordinates": [
    [
      [0.1234, 0.1234],
      [0.2345, 0.1234],
      [0.2345, 0.2345],
      [0.1234, 0.2345],
      [0.1234, 0.1234]
    ]
  ]
}
`,
		},
		{
			bbox: BoundingBox{0.8765, 0.8765, 9.8765, 9.8765},
			expectedGeoJson: `
{
  "type": "Polygon",
  "coordinates": [
    [
      [0.8765, 0.8765],
      [9.8765, 0.8765],
      [9.8765, 9.8765],
      [0.8765, 9.8765],
      [0.8765, 0.8765]
    ]
  ]
}
`,
		},
	}

	for i, tc := range testCases {
		geoJson := tc.bbox.AsGeoJSON()
		if !reflect.DeepEqual(tc.expectedGeoJson, geoJson) {
			t.Errorf("[%v] geojson, expected %v got %v", i, tc.expectedGeoJson, geoJson)
		}
	}
}
