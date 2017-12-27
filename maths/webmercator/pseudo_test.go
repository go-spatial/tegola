package webmercator_test

import (
	"math"
	"testing"

	"github.com/terranodo/tegola/maths/webmercator"
)

func float64Equal(f1, f2, delta float64) (equal bool) {
	// Checks if f1 == f2 within a difference of delta
	if math.Abs(f1-f2) < delta {
		return true
	}
	return false
}

func TestPToLonLat(t *testing.T) {
	// Acceptable fuzziness in equality between longitude/latitude float values
	floatDelta := 0.00000001

	// Expected values according to 'https://mygeodata.cloud/cs2cs/' and/or 'https://twcc.fr/#'
	type TestCase struct {
		// WebMercator x, y
		wmX float64
		wmY float64
		// Expected longitude, latitude
		expectedLng float64
		expectedLat float64
	}

	testCases := []TestCase{
		{
			wmX:         -16123932.495,
			wmY:         -11818999.062,
			expectedLng: -144.84375,
			expectedLat: -72.18180355624852,
		},
		{
			wmX:         15341217.325,
			wmY:         -6339992.874,
			expectedLng: 137.8125,
			expectedLat: -49.38237278700955,
		},
		{
			wmX:         15615167.634,
			wmY:         8257645.04,
			expectedLng: 140.2734375,
			expectedLat: 59.355596110016315,
		},
		{
			wmX:         -11310234.201,
			wmY:         7709744.421,
			expectedLng: -101.6015625,
			expectedLat: 56.75272287205736,
		},
	}

	for i, tc := range testCases {
		var resultLngLat []float64
		resultLngLat, err := webmercator.PToLonLat(tc.wmX, tc.wmY)
		if err != nil {
			t.Errorf("[%v] Error in webmercator.PToLonLat(): %v", i, err)
		}

		resultLng := resultLngLat[0]
		resultLat := resultLngLat[1]
		if !float64Equal(resultLng, tc.expectedLng, floatDelta) ||
			!float64Equal(resultLat, tc.expectedLat, floatDelta) {

			t.Errorf("[%v] Converted (lng,lat) doesn't match expected: (%v,%v) != (%v,%v)",
				i, resultLng, resultLat, tc.expectedLng, tc.expectedLat)
		}
	}
}

func TestPToXY(t *testing.T) {
	// Acceptable fuzziness in equality between x,y float values
	floatDelta := 0.001

	// Expected values according to 'https://mygeodata.cloud/cs2cs/' and/or 'https://twcc.fr/#'
	type TestCase struct {
		// longitude, latitude
		lng float64
		lat float64
		// Expected x, y
		expectedX float64
		expectedY float64
	}

	testCases := []TestCase{
		{
			lng:       -144.84375,
			lat:       -72.18180355624852,
			expectedX: -16123932.495,
			expectedY: -11818999.062,
		},
		{
			lng:       137.8125,
			lat:       -49.38237278700955,
			expectedX: 15341217.325,
			expectedY: -6339992.874,
		},
		{
			lng:       140.2734375,
			lat:       59.355596110016315,
			expectedX: 15615167.634,
			expectedY: 8257645.04,
		},
		{
			lng:       -101.6015625,
			lat:       56.75272287205736,
			expectedX: -11310234.201,
			expectedY: 7709744.421,
		},
	}

	for i, tc := range testCases {
		var resultXY []float64
		resultXY, err := webmercator.PToXY(tc.lng, tc.lat)
		if err != nil {
			t.Errorf("[%v] Error in webmercator.PToLonLat(): %v", i, err)
		}

		resultX := resultXY[0]
		resultY := resultXY[1]
		if !float64Equal(resultX, tc.expectedX, floatDelta) ||
			!float64Equal(resultY, tc.expectedY, floatDelta) {

			t.Errorf("[%v] Converted (x,y) doesn't match expected: (%v,%v) != (%v,%v)",
				i, resultX, resultY, tc.expectedX, tc.expectedY)
		}
	}
}
