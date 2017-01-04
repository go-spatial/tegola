package clip

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"testing"

	"github.com/gdey/tbl"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/clip/internal/draw"
)

type PolygonTestCase struct {
	desc string
	// Index of the region to use for clipping.
	ridx int
	// Starting Polygon to clip
	p basic.Polygon

	// Expected Polygons
	eps []basic.Polygon

	// Expected error
	eerr error
	// This is for the debug image to be produced. Instead of calculating it, we just tell the system how big the image needs to be to hold all values.
	min, max maths.Pt
}

var SegmentColor = color.RGBA{0xA0, 0xA0, 0xA0, 0xFF}
var ExpectedColor = color.RGBA{0xE9, 0x7F, 0x02, 0xFF}
var GotColor = color.RGBA{0x8A, 0x9B, 0x0F, 0xFF}
var RegionColor = color.RGBA{0xD0, 0xD0, 0xD0, 0xFF}
var RegionAColor = color.RGBA{0x00, 0x00, 0x00, 0xFF}

func (tc *PolygonTestCase) Region() Region { return testRegion[tc.ridx] }
func (tc *PolygonTestCase) DrawTestCase(got []basic.Polygon, filename string) {
	log.Println("Creating png: ", filename)
	m := draw.NewImage(tc.min, tc.max)
	r := tc.Region()
	draw.Region(
		m,
		int(tc.min.X),
		int(tc.min.Y),
		r.ClipRegion(maths.Clockwise),
		RegionAColor,
	)
	r.Min.X -= float64(r.Extant)
	r.Min.Y -= float64(r.Extant)
	r.Max.X += float64(r.Extant)
	r.Max.Y += float64(r.Extant)
	draw.Region(
		m,
		int(tc.min.X),
		int(tc.min.Y),
		r.ClipRegion(maths.Clockwise),
		RegionColor,
	)
	draw.Polygon(m, tc.min, &tc.p, SegmentColor)
	for _, ep := range tc.eps {
		draw.Polygon(m, tc.min, &ep, ExpectedColor)
	}
	for _, p := range got {
		draw.Polygon(m, tc.min, &p, GotColor)
	}
	draw.Orgin(m, tc.min, nil)
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Error creating file %v: %v\n", filename, err)
		return
	}
	png.Encode(f, m)
}

func checkLine(want, got tegola.LineString) (string, bool) {
	wantPts := want.Subpoints()
	gotPts := got.Subpoints()
	if len(wantPts) != len(gotPts) {
		return fmt.Sprintf("Number of elements do not match want %v got %v", len(wantPts), len(gotPts)), false
	}
	for i, pt := range wantPts {
		if pt.X() != gotPts[i].X() || pt.Y() != gotPts[i].Y() {
			return fmt.Sprintf(
					"Point %v in Does not match, want: (%v,%v) got:(%v,%v)", i,
					pt.X(),
					pt.Y(),
					gotPts[i].X(),
					gotPts[i].Y()),
				false
		}
	}
	return "", true
}

func checkPolygon(want, got tegola.Polygon) (string, bool) {
	gotLines := got.Sublines()
	wantLines := want.Sublines()
	if len(wantLines) != len(gotLines) {
		return fmt.Sprintf("Number of elements do not match want %v got %v", len(wantLines), len(gotLines)), false
	}
	for i, l := range wantLines {
		if desc, ok := checkLine(l, gotLines[i]); !ok {
			return fmt.Sprintf("For line %v: %v", i, desc), ok
		}
	}
	return "", true
}

func TestClipPolygon(t *testing.T) {

	test := tbl.Cases(
		PolygonTestCase{
			desc: "Basic Polygon contain clip region.",
			p: basic.NewPolygon(
				[]maths.Pt{
					maths.Pt{-1, -1},
					maths.Pt{11, -1},
					maths.Pt{11, 11},
					maths.Pt{-1, 11},
				}),
			eps: []basic.Polygon{
				basic.NewPolygon(
					[]maths.Pt{
						maths.Pt{-1, -1},
						maths.Pt{11, -1},
						maths.Pt{11, 11},
						maths.Pt{-1, 11},
					}),
			},
			// For the image to be drawn.
			min: maths.Pt{-5, -5},
			max: maths.Pt{15, 15},
		},
		PolygonTestCase{
			desc: "Basic Polygon with a cut out.",
			// For the image to be drawn.
			min: maths.Pt{-10, -10},
			max: maths.Pt{20, 20},
			p: basic.NewPolygon(
				[]maths.Pt{
					maths.Pt{-1, -1},
					maths.Pt{14, -1},
					maths.Pt{14, 14},
					maths.Pt{-1, 14},
				},
				[]maths.Pt{
					maths.Pt{2, 2},
					maths.Pt{2, 10},
					maths.Pt{10, 10},
					maths.Pt{10, 2},
				}),
			eps: []basic.Polygon{
				basic.NewPolygon(
					[]maths.Pt{
						maths.Pt{-1, 11},
						maths.Pt{-1, -1},
						maths.Pt{11, -1},
						maths.Pt{11, 11},
					},
					[]maths.Pt{
						maths.Pt{10, 2},
						maths.Pt{2, 2},
						maths.Pt{2, 10},
						maths.Pt{10, 10},
					},
				),
			},
		},
		PolygonTestCase{
			desc: "Basic Polygon with two cut outs.",
			// For the image to be drawn.
			min: maths.Pt{-10, -10},
			max: maths.Pt{20, 20},
			p: basic.NewPolygon(
				[]maths.Pt{
					{-5, -5},
					{14, -5},
					{14, 14},
					{-5, 14},
				},
				[]maths.Pt{
					{-2, -2},
					{-2, 3},
					{3, 3},
					{3, -2},
				},
				[]maths.Pt{
					{4, 1},
					{4, 12},
					{12, 12},
					{12, 1},
				},
			),
			eps: []basic.Polygon{
				basic.NewPolygon(
					[]maths.Pt{
						{-1, 11},
						{-1, -1},
						{11, -1},
						{11, 11},
					},
					[]maths.Pt{
						{0, 3},
						{3, 3},
						{3, 0},
						{0, 0},
					},
					[]maths.Pt{
						{10, 1},
						{4, 1},
						{4, 10},
						{10, 10},
					},
				),
			},
		},
		PolygonTestCase{
			desc: "Basic Polygon with two cut outs.",
			// For the image to be drawn.
			min: maths.Pt{-10, -10},
			max: maths.Pt{20, 20},
			p: basic.NewPolygon(
				[]maths.Pt{
					{-5, -2},
					{3, -2},
					{3, 3},
					{-2, 3},
					{-2, 12},
					{4, 12},
					{4, 0},
					{12, 0},
					{12, 13},
					{-5, 13},
				},
				[]maths.Pt{
					{-2, -1},
					{-2, 2},
					{2, 2},
					{2, -1},
				},
				[]maths.Pt{
					{5, 2},
					{5, 11},
					{11, 11},
					{11, 2},
				},
			),
			eps: []basic.Polygon{
				basic.NewPolygon(
					[]maths.Pt{
						{3, -1},
						{3, 3},
						{-1, 3},
						{-1, -1},
					},
					[]maths.Pt{
						{0, 2},
						{2, 2},
						{2, 0},
						{0, 0},
					},
				),
				basic.NewPolygon(
					[]maths.Pt{
						{4, 11},
						{4, 0},
						{11, 0},
						{11, 11},
					},
					[]maths.Pt{
						{10, 2},
						{5, 2},
						{5, 10},
						{10, 10},
					},
				),
			},
		},
	)
	test.Run(func(i int, tc PolygonTestCase) {
		var drawPng bool
		t.Log("Starting test ", i)
		r := tc.Region()
		got, err := Polygon(&tc.p, r.Min, r.Max, r.Extant)
		if err != tc.eerr {
			t.Errorf("Did not get expected error: want : %v got %v", err, tc.eerr)
			drawPng = true
			goto DRAW_IMAGE
		}
		t.Logf("Got %#v :  %v", got, err)
		if len(tc.eps) != len(got) {
			t.Errorf("For test(%v-%v): wanted %v polygons got %v", i, tc.desc, len(tc.eps), len(got))
			drawPng = true
			goto DRAW_IMAGE
		}
		for j, ep := range tc.eps {
			if desc, ok := checkPolygon(&ep, &got[j]); !ok {
				drawPng = true
				t.Errorf("For test(%v-%v) polygon(%v) : %v", i, tc.desc, j, desc)
			}
		}
	DRAW_IMAGE:
		if drawPng || *showPng {
			tc.DrawTestCase(got, fmt.Sprintf("tstcase_%v.png", i))
		}

	})
}
