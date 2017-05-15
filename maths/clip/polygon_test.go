package clip

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"testing"

	"github.com/gdey/tbltest"
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
	draw.Origin(m, tc.min, nil)
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

	test := tbltest.Cases(

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
		PolygonTestCase{ // 1
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
						maths.Pt{2, 2},
						maths.Pt{2, 10},
						maths.Pt{10, 10},
						maths.Pt{10, 2},
					},
				),
			},
		},
		PolygonTestCase{ // 2
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
		PolygonTestCase{ // 3
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

		PolygonTestCase{ // 4
			desc: "Polygon from osm_bonn test.",
			// For the image to be drawn.
			min:  maths.Pt{-2, -2},
			max:  maths.Pt{4098, 4098},
			ridx: 11,
			p: basic.NewPolygon(
				[]maths.Pt{
					{4038, 1792}, // inside
					{4042, 1786}, // inside
					{4035, 1782}, // inside
					{4047, 1762}, // inside
					{4054, 1767}, // inside
					{4060, 1756}, // inside
					{4064, 1758}, // inside
					{4067, 1754}, // inside
					{4056, 1748}, // inside
					{4070, 1726}, // inside
					{4061, 1720}, // inside
					{4072, 1702}, // inside
					{4083, 1709}, // inside
					{4101, 1720}, // crosses (outside) 4098,1718
					{4089, 1740}, // crosses (inside) 4098,1725
					{4098, 1746}, // crosses (outside?) tricky part (insert in two points one as outbound and one as inbound)
					{4088, 1763}, // inside
					{4080, 1759}, // inside
					{4076, 1765}, // inside
					{4066, 1782}, // inside
					{4070, 1785}, // inside
					{4058, 1804}, // inside
					//{4038, 1792},
				},
			),
			eps: []basic.Polygon{
				basic.NewPolygon(
					[]maths.Pt{
						{4098, 1725},
						{4089, 1740},
						{4098, 1746},
						{4088, 1763},
						{4080, 1759},
						{4076, 1765},
						{4066, 1782},
						{4070, 1785},
						{4058, 1804},
						{4038, 1792},
						{4042, 1786},
						{4035, 1782},
						{4047, 1762},
						{4054, 1767},
						{4060, 1756},
						{4064, 1758},
						{4067, 1754},
						{4056, 1748},
						{4070, 1726},
						{4061, 1720},
						{4072, 1702},
						{4083, 1709},
						{4098, 1718},
					},
				),
			},
		},

		PolygonTestCase{ // 5
			desc: "Polygon from osm_bonn test 2.",
			// For the image to be drawn.
			min:  maths.Pt{-2, -2},
			max:  maths.Pt{4098, 4098},
			ridx: 11,
			p: basic.NewPolygon(
				[]maths.Pt{
					{-160, 1205}, // outside
					{-154, 1187}, // outside
					{-122, 1146}, // outside
					{-91, 1113},  // outside
					{-60, 1086},  // outside
					{-33, 1072},  // outside

					{-2, 1063},  // This is an intersection point on the boundary.
					{22, 1059},  // inside
					{47, 1059},  // inside
					{74, 1080},  // inside
					{101, 1115}, // inside
					{137, 1176}, // inside
					{139, 1187}, // inside
					{131, 1196}, // inside
					{116, 1208}, // inside
					{89, 1226},  // inside
					{67, 1238},  // inside
					{50, 1246},  // inside
					{4, 1248},   // inside
					// intersection here {-2, 1249}
					{-15, 1254}, // outside
					{-43, 1258},
					{-73, 1252},
					{-83, 1247},
					{-72, 1211},
					{-65, 1199},
					{-59, 1187},
					{-60, 1176},
					{-69, 1171},
					{-80, 1171},
					{-91, 1179},
					{-111, 1223},
					{-118, 1226},
					{-150, 1218},
					//{-160, 1205},
				},
			),
			eps: []basic.Polygon{
				basic.NewPolygon(
					[]maths.Pt{
						{-2, 1063},
						{22, 1059},
						{47, 1059},
						{74, 1080},
						{101, 1115},
						{137, 1176},
						{139, 1187},
						{131, 1196},
						{116, 1208},
						{89, 1226},
						{67, 1238},
						{50, 1246},
						{4, 1248},
						{-2, 1249},
					},
				),
			},
		},
	)
	test.RunOrder = "5"
	test.Run(func(i int, tc PolygonTestCase) {
		var drawPng bool
		t.Log("Starting test ", i)
		r := tc.Region()
		got, err := Polygon(tc.p, r.Min, r.Max, r.Extant)
		if err != tc.eerr {
			t.Errorf("Did not get expected error: want : %v got %v", err, tc.eerr)
			drawPng = true
			goto DRAW_IMAGE
		}
		t.Logf("Got %#v :  %v", got, err)
		if len(tc.eps) != len(got) {
			t.Errorf("For test(%v-%v): wanted %v polygons got %v", i, tc.desc, len(tc.eps),
				len(got))
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
			// tc.DrawTestCase(got, fmt.Sprintf("tstcase_%v.png", i))
		}
	})
}
