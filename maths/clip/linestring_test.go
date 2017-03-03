package clip

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/clip/internal/draw"
	"github.com/terranodo/tegola/maths/clip/region"
)

var showPng = flag.Bool("drawPNG", false, "Draw the PNG for the test cases even if the testcase passes.")

func drawTestCase(tc *TestCase, got [][]float64, filename string) {
	log.Println("Creating png: ", filename)

	s := tc.subject
	r := tc.ClipRegion().LineString()
	minx, miny, maxx, maxy := draw.Minmax(s, int(r[0]), int(r[1]), int(r[2]), int(r[3]))

	for _, i := range got {
		minx, miny, maxx, maxy = draw.Minmax(i, minx, miny, maxx, maxy)
	}
	min := maths.Pt{float64(minx), float64(miny)}
	max := maths.Pt{float64(maxx), float64(maxy)}
	m := draw.NewImage(min, max)
	draw.Region(m, minx, miny, tc.ClipRegion(), RegionColor)
	draw.Segment(m, minx, miny, tc.subject, SegmentColor)

	for _, i := range tc.e {
		draw.Segment(m, minx, miny, i, ExpectedColor)
	}
	for _, i := range got {
		draw.Segment(m, minx, miny, i, GotColor)
	}
	draw.Orgin(m, min, nil)
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Error creating file %v: %v\n", filename, err)
		return
	}
	png.Encode(f, m)
}

type Region struct {
	Max, Min maths.Pt
	Extant   int
}

var testRegion = []Region{
	Region{Min: maths.Pt{X: 0, Y: 0}, Max: maths.Pt{X: 10, Y: 10}, Extant: 1},
	Region{Min: maths.Pt{X: 2, Y: 2}, Max: maths.Pt{X: 8, Y: 8}, Extant: 1},
	Region{Min: maths.Pt{X: -1, Y: -1}, Max: maths.Pt{X: 11, Y: 11}, Extant: 1},
	Region{Min: maths.Pt{X: -2, Y: -2}, Max: maths.Pt{X: 12, Y: 12}, Extant: 1},
	Region{Min: maths.Pt{X: -3, Y: -3}, Max: maths.Pt{X: 13, Y: 13}, Extant: 1},
	Region{Min: maths.Pt{X: -4, Y: -4}, Max: maths.Pt{X: 14, Y: 14}, Extant: 1},
	Region{Min: maths.Pt{X: 5, Y: 1}, Max: maths.Pt{X: 7, Y: 3}, Extant: 1},
	Region{Min: maths.Pt{X: 0, Y: 5}, Max: maths.Pt{X: 2, Y: 7}, Extant: 1},
	Region{Min: maths.Pt{X: -1, Y: 4}, Max: maths.Pt{X: 5, Y: 8}, Extant: 1},
	Region{Min: maths.Pt{X: 5, Y: 2}, Max: maths.Pt{X: 11, Y: 9}, Extant: 1},
	Region{Min: maths.Pt{X: -1, Y: -1}, Max: maths.Pt{X: 11, Y: 11}, Extant: 1},
	Region{Min: maths.Pt{X: -2, Y: -2}, Max: maths.Pt{X: 4098, Y: 4098}, Extant: 0},
}

func (r *Region) ClipRegion(w maths.WindingOrder) *region.Region {
	if r == nil {
		return region.New(w, maths.Pt{}, maths.Pt{100, 100})
	}
	return region.New(w, r.Min, r.Max)
}

type TestCase struct {
	region  Region
	winding maths.WindingOrder
	subject []float64
	e       [][]float64
}

func (tc *TestCase) ClipRegion() *region.Region {
	return tc.region.ClipRegion(tc.winding)
}

func TestCliplinestring(t *testing.T) {

	test := tbltest.Cases(
		TestCase{
			region:  testRegion[0],
			winding: maths.Clockwise,
			subject: []float64{-2, 1, 2, 1, 2, 2, -1, 2, -1, 11, 2, 11, 2, 4, 4, 4, 4, 13, -2, 13},
			e: [][]float64{
				{0, 1, 2, 1, 2, 2, 0, 2},
				{2, 10, 2, 4, 4, 4, 4, 10},
			},
		},
		TestCase{
			region:  testRegion[0],
			winding: maths.Clockwise,
			subject: []float64{-2, 1, 12, 1, 12, 2, -1, 2, -1, 11, 2, 11, 2, 4, 4, 4, 4, 13, -2, 13},
			e: [][]float64{
				{0, 1, 10, 1, 10, 2, 0, 2},
				{2, 10, 2, 4, 4, 4, 4, 10},
			},
		},
		TestCase{
			region:  testRegion[0],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				{0, 9, 10, 9, 10, 2, 5, 2, 5, 8, 0, 8},
				{0, 4, 3, 4, 3, 1, 0, 1},
			},
		},
		TestCase{
			region:  testRegion[1],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				{8, 2, 5, 2, 5, 8, 8, 8},
				{2, 4, 3, 4, 3, 2, 2, 2},
			},
		},
		TestCase{
			region:  testRegion[2],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				{-1, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8},
				{-1, 4, 3, 4, 3, 1, -1, 1},
			},
		},
		TestCase{
			region:  testRegion[3],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				{-2, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1, -2, 1},
			},
		},
		TestCase{
			region:  testRegion[4],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				[]float64{-3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1, -3, 1},
			},
		},
		TestCase{
			region:  testRegion[5],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				[]float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			},
		},
		TestCase{
			region:  testRegion[6],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				[]float64{7, 2, 5, 2, 5, 3, 7, 3},
			},
		},
		TestCase{
			region:  testRegion[7],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e:       [][]float64{},
		},
		TestCase{
			region:  testRegion[8],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e:       [][]float64{},
		},
		TestCase{
			region:  testRegion[9],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				[]float64{5, 9, 11, 9, 11, 2, 5, 2, 5, 8},
			},
		},
		TestCase{
			region:  testRegion[9],
			winding: maths.CounterClockwise,
			subject: []float64{-3, 1, -3, 10, 12, 10, 12, 1, 4, 1, 4, 8, -1, 8, -1, 4, 3, 4, 3, 1},
			e: [][]float64{
				[]float64{5, 2, 5, 9, 11, 9, 11, 2},
			},
		},
		TestCase{
			region:  testRegion[0],
			winding: maths.CounterClockwise,
			subject: []float64{-3, -3, -3, 10, 12, 10, 12, 1, 4, 1, 4, 8, -1, 8, -1, 4, 3, 4, 3, 3},
			e: [][]float64{
				[]float64{0, 10, 10, 10, 10, 1, 4, 1, 4, 8, 0, 8},
				[]float64{0, 4, 3, 4, 3, 3, 0, 0},
			},
		},
		TestCase{
			region:  testRegion[10],
			winding: maths.Clockwise,
			subject: []float64{-1, -1, 12, -1, 12, 12, -1, 12},
			e: [][]float64{
				[]float64{-1, 11, -1, -1, 11, -1, 11, 11},
			},
		},
		TestCase{
			region:  testRegion[11],
			winding: maths.Clockwise,
			subject: []float64{7848, 19609, 7340, 18835, 6524, 17314, 6433, 17163, 5178, 15057, 5147, 15006, 4680, 14226, 3861, 12766, 2471, 10524, 2277, 10029, 1741, 8281, 1655, 8017, 1629, 7930, 1437, 7368, 973, 5481, 325, 4339, -497, 3233, -1060, 2745, -1646, 2326, -1883, 2156, -2002, 2102, -2719, 1774, -3638, 1382, -3795, 1320, -5225, 938, -6972, 295, -7672, -88, -8243, -564, -8715, -1112, -9019, -1573, -9235, -2067, -9293, -2193, -9408, -2570, -9823, -4630, -10118, -5927, -10478, -7353, -10909, -8587, -11555, -9743, -11837, -10005, -12277, -10360, -13748, -11189, -14853, -12102, -15806, -12853, -16711, -13414},
			e: [][]float64{
				[]float64{-2, 3899, 145, 4098},
			},
		},
	)

	test.Run(func(i int, tc TestCase) {
		var drawPng bool
		t.Log("Starting test ", i)
		got, _ := linestring(tc.winding, tc.subject, tc.region.Min, tc.region.Max)
		if len(tc.e) != len(got) {
			t.Errorf("Test %v: Expected number of slices to be %v got: %v -- %+v", i, len(tc.e), len(got), got)
			drawTestCase(&tc, got, fmt.Sprintf("testcase%v.png", i))
			return
		}
		for j := range tc.e {

			if len(tc.e[j]) != len(got[j]) {
				drawPng = true
				t.Errorf("Test %v: Expected slice %v to have %v items got: %v -- %+v", i, i, len(tc.e[j]), len(got[j]), got[j])
				continue
			}
			for k := 0; k < len(tc.e[j])/2; k++ {
				k1 := k * 2
				k2 := k1 + 1
				if (tc.e[j][k1] != got[j][k1]) || (tc.e[j][k2] != got[j][k2]) {
					drawPng = true
					t.Errorf("Test %v: Expected Sice: %v  item: %v to be ( %v %v ) got: ( %v %v)", i, j, k, tc.e[j][k1], tc.e[j][k2], got[j][k1], got[j][k2])
				}
			}
		}
		_ = drawPng
		/*
			if drawPng || *showPng {
				drawTestCase(&tc, got, fmt.Sprintf("testcase%v.png", i))
			}
		*/

	})
}
