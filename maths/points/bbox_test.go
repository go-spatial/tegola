package points

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/terranodo/tegola"
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
		assert.Equal(t, tc.disjoint, disjoint, fmt.Sprintf("TestCase[%v] failed", i))
	}
}

func TestConvertSrid(t *testing.T) {
	type TestCase struct {
		bbox         BoundingBox
		fromSrid     int
		toSrid       int
		expectedBbox BoundingBox
	}

	testCases := [...]TestCase{
		TestCase{
			bbox:     BoundingBox{26.0, 32.0, 27.0, 33.0},
			fromSrid: tegola.WebMercator,
			toSrid:   tegola.WGS84,
			expectedBbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
		},
		TestCase{
			bbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
			fromSrid:     tegola.WGS84,
			toSrid:       tegola.WebMercator,
			expectedBbox: BoundingBox{26.0, 32.0, 27.0, 33.0},
		},
		TestCase{
			bbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
			fromSrid: tegola.WebMercator,
			toSrid:   tegola.WebMercator,
			expectedBbox: BoundingBox{0.00023356197387107555, 0.0002874608909118022,
				0.0002425451267122708, 0.00029644404375279597},
		},
		TestCase{
			bbox:         BoundingBox{26.0, 32.0, 27.0, 33.0},
			fromSrid:     tegola.WGS84,
			toSrid:       tegola.WGS84,
			expectedBbox: BoundingBox{26.0, 32.0, 27.0, 33.0},
		},
	}

	floatDelta := 0.00000001
	for i, tc := range testCases {
		convertedBBox := tc.bbox.ConvertSrid(tc.fromSrid, tc.toSrid)
		failMsg := fmt.Sprintf("TestCase[%v]: %v != %v", i, convertedBBox, tc.expectedBbox)
		for j := 0; j < 4; j++ {
			assert.InDelta(t, tc.expectedBbox[j], convertedBBox[j], floatDelta, failMsg)
		}
	}
}
