package atlas_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/atlas"
)

func TestMapEnableLayersByZoom(t *testing.T) {
	testcases := []struct {
		atlasMap atlas.Map
		zoom     int
		expected atlas.Map
	}{
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
			zoom: 5,
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: true,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
		},
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
			zoom: 2,
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.EnableLayersByZoom(tc.zoom)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}

func TestMapEnableLayersByName(t *testing.T) {
	testcases := []struct {
		atlasMap atlas.Map
		name     string
		expected atlas.Map
	}{
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: true,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: true,
					},
				},
			},
			name: "layer1",
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: true,
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.EnableLayersByName(tc.name)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}

func TestMapClosestCell(t *testing.T) {
	testcases := []struct {
		atlasMap                        atlas.Map
		zoom                            int
		minx, miny                      float64
		expectedZ, expectedX, expectedY int
	}{
		{
			atlasMap:  atlas.NewWGS84Map("test-map"),
			zoom:      0,
			minx:      -180,
			miny:      -90,
			expectedZ: 0,
			expectedX: 0,
			expectedY: 0,
		},
		{
			atlasMap:  atlas.NewWGS84Map("test-map"),
			zoom:      2,
			minx:      84,
			miny:      17,
			expectedZ: 2,
			expectedX: 6,
			expectedY: 2,
		},
	}

	for i, tc := range testcases {
		z, x, y := tc.atlasMap.ClosestCell(tc.zoom, tc.minx, tc.miny)

		if z != tc.expectedZ || x != tc.expectedX || y != tc.expectedY {
			t.Errorf("testcase (%v) failed. output (Z:%v, X:%v, Y:%v) does not match expected (Z:%v, X:%v, Y:%v)", i, z, x, y, tc.expectedZ, tc.expectedX, tc.expectedY)
		}
	}
}

func TestMapCell(t *testing.T) {
	testcases := []struct {
		atlasMap                        atlas.Map
		minx, miny, maxx, maxy          float64
		expectedZ, expectedX, expectedY int
	}{
		{
			atlasMap:  atlas.NewWGS84Map("test-map"),
			minx:      -180,
			miny:      -90,
			maxx:      0,
			maxy:      90,
			expectedZ: 0,
			expectedX: 0,
			expectedY: 0,
		},
		{
			atlasMap:  atlas.NewWGS84Map("test-map"),
			minx:      -45,
			miny:      -45,
			maxx:      0,
			maxy:      0,
			expectedZ: 2,
			expectedX: 3,
			expectedY: 1,
		},
	}

	for i, tc := range testcases {
		z, x, y := tc.atlasMap.Cell(tc.minx, tc.miny, tc.maxx, tc.maxy)

		if z != tc.expectedZ || x != tc.expectedX || y != tc.expectedY {
			t.Errorf("testcase (%v) failed. output (Z:%v, X:%v, Y:%v) does not match expected (Z:%v, X:%v, Y:%v)", i, z, x, y, tc.expectedZ, tc.expectedX, tc.expectedY)
		}
	}
}

func TestMapClosestLevel(t *testing.T) {
	testcases := []struct {
		atlasMap      atlas.Map
		res           float64
		expectedLevel int
	}{
		{
			atlasMap:      atlas.NewWGS84Map("test-map"),
			res:           0.703125,
			expectedLevel: 0,
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.ClosestLevel(tc.res)

		if output != tc.expectedLevel {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expectedLevel)
		}
	}
}

func TestMapLevel(t *testing.T) {
	testcases := []struct {
		atlasMap      atlas.Map
		res           float64
		expectedLevel int
	}{
		{
			atlasMap:      atlas.NewWGS84Map("test-map"),
			res:           0.703125,
			expectedLevel: 0,
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.Level(tc.res)

		if output != tc.expectedLevel {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expectedLevel)
		}
	}
}

func TestResolution(t *testing.T) {
	testcases := []struct {
		atlasMap               atlas.Map
		minx, miny, maxx, maxy float64
		expectedRes            float64
	}{
		{
			atlasMap:    atlas.NewWGS84Map("test-map"),
			minx:        -180,
			miny:        -90,
			maxx:        0,
			maxy:        90,
			expectedRes: 0.703125,
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.Resolution(tc.minx, tc.miny, tc.maxx, tc.maxy)

		if output != tc.expectedRes {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expectedRes)
		}
	}
}
