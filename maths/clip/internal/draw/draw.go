package draw

import (
	"fmt"
	"io"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/draw/svg"
	"github.com/go-spatial/tegola/maths/clip/region"
)

/*
const PixelWidth = 10

// Drawing routines.
func Minmax(s []float64, mix, miy, mx, my int) (minx, miny, maxx, maxy int) {
	minx = mix
	miny = miy
	maxx = mx
	maxy = my
	for i := 0; i < len(s); i += 2 {
		if int(s[i]) > maxx {
			maxx = int(s[i])
		}
		if int(s[i]) < minx {
			minx = int(s[i])
		}
		if int(s[i+1]) > maxy {
			maxy = int(s[i+1])
		}
		if int(s[i+1]) < miny {
			miny = int(s[i+1])
		}
	}
	return minx - 1, miny - 1, maxx + 1, maxy + 1
}

func line(img *image.RGBA, pt1, pt2 image.Point, c color.RGBA) {

	if pt1.X == pt2.X && pt1.Y == pt2.Y {
		img.Set(pt1.X, pt1.Y, c)
		return
	}

	sx := pt1.X
	mx := pt2.X
	if pt2.X < pt1.X {
		sx = pt2.X
		mx = pt1.X
	}

	sy := pt1.Y
	my := pt2.Y
	if pt2.Y < pt1.Y {
		sy = pt2.Y
		my = pt1.Y
	}

	img.Set(sx, sy, c)
	img.Set(mx, my, c)
	xdelta := mx - sx

	// We have a veritcal line.
	if xdelta == 0 {
		for y := sy; y < my; y++ {
			img.Set(sx, y, c)
		}
		return
	}
	ydelta := my - sy
	if ydelta == 0 {
		for x := sx; x < mx; x++ {
			img.Set(x, sy, c)
		}
		return
	}
	m := int(ydelta / xdelta)
	b := int(sy - (m * sx))
	//y = mx+b
	for x := sx; x < mx; x++ {
		y := (m * x) + b
		img.Set(x, y, c)
	}
}

func NewImage(min, max maths.Pt) *image.RGBA {
	delta := max.Delta(min)
	delta.X *= PixelWidth
	delta.Y *= PixelWidth
	m := image.NewRGBA(image.Rect(0, 0, int(delta.X), int(delta.Y)))
	for x := 0; x < int(delta.X); x++ {
		for y := 0; y < int(delta.Y); y++ {
			m.Set(x, y, color.RGBA{255, 255, 255, 255})
			if y == 0 || x == 0 || y == int(delta.Y-1) || x == int(delta.X-1) {
				continue
			}
			if y%PixelWidth == 0 || x%PixelWidth == 0 {
				m.Set(x, y, color.RGBA{0xF0, 0xF0, 0xF0, 0xFF})
			}
		}
	}
	return m
}

func ScaleToPoint(minx, miny int, x, y float64) image.Point {
	sx, sy := (int(x)-minx)*PixelWidth, (int(y)-miny)*PixelWidth
	return image.Pt(sx, sy)
}

func Segment(img *image.RGBA, minx, miny int, s []float64, c color.RGBA) {
	if len(s) == 0 {
		return
	}
	pt := ScaleToPoint(minx, miny, s[len(s)-2], s[len(s)-1])
	for i := 0; i < len(s); i += 2 {
		npt := ScaleToPoint(minx, miny, s[i], s[i+1])
		line(img, pt, npt, c)
		pt = npt
	}
}
func Region(img *image.RGBA, minx, miny int, clipr *region.Region, c color.RGBA) {
	r := clipr.LineString()
	// log.Printf("Drawing region: %v", r)
	Segment(img, minx, miny, r, c)
}

func LineString(img *image.RGBA, min maths.Pt, line tegola.LineString, c color.RGBA) {
	var ls []float64
	for _, p := range line.Subpoints() {
		ls = append(ls, p.X(), p.Y())
	}
	Segment(img, int(min.X), int(min.Y), ls, c)
}

func Polygon(img *image.RGBA, min maths.Pt, polygon tegola.Polygon, c color.RGBA) {
	for _, l := range polygon.Sublines() {
		LineString(img, min, l, c)
	}
}

func Origin(img *image.RGBA, min maths.Pt, c *color.RGBA) {
	cc := color.RGBA{255, 0, 255, 100}
	if c != nil {
		cc = *c
	}
	cc.A = 255
	orn := ScaleToPoint(int(min.X), int(min.Y), 0, 0)
	img.Set(orn.X, orn.Y, cc)
}
*/

func DrawPolygonTest(writer io.Writer, w, h int, r *region.Region, original tegola.Polygon, expected []tegola.Polygon, got []tegola.Polygon) {

	reg := svg.MinMax{
		MinX: int64(r.Min().X),
		MinY: int64(r.Min().Y),
		MaxX: int64(r.Max().X),
		MaxY: int64(r.Max().Y),
	}
	mm := svg.MinMax{reg.MaxX, reg.MaxY, reg.MinX, reg.MinY}
	mm.OfGeometry(original)

	mm.ExpandBy(100)

	canvas := &svg.Canvas{
		Board:  mm,
		Region: svg.MinMax{0, 0, 4096, 4096},
	}

	canvas.Init(writer, w, h, false)

	canvas.DrawGeometry(original, "original", "fill:yellow;opacity:1", "fill:black", true)
	canvas.DrawRegion(true)
	canvas.Gid("expected")
	for i, p := range expected {
		canvas.DrawGeometry(p, fmt.Sprintf("%v_expected", i), "fill:green;opacity:0.5", "fill:green;opacity:0.5", false)
	}
	canvas.Gend()
	canvas.Gid("got")
	for i, p := range got {
		canvas.DrawGeometry(p, fmt.Sprintf("%v_got", i), "fill:blue;opacity:0.5", "fill:blue;opacity:0.5", true)
	}
	canvas.Gend()
	canvas.End()
}
