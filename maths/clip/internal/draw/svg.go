package draw

import (
	"fmt"
	"io"

	"github.com/ajstarks/svgo"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/clip/region"
)

type SVG struct {
	Filename string
}

func DrawPoint(canvas *svg.SVG, x, y int, fill string) {

	canvas.Gstyle("text-anchor:middle;font-size:8;fill:white;stroke:black")
	canvas.Circle(x, y, 1, "fill:"+fill)
	canvas.Text(x, y+5, fmt.Sprintf("(%v %v)", x, y))
	canvas.Gend()
}

func DrawGrid(canvas *svg.SVG, x, y, w, h, n int, label bool, style string) {
	canvas.Gstyle(style)
	// Draw all the horizontal and vertical lines.
	for i := x; i < w; i += n {
		canvas.Line(i, y, i, h)
	}
	for i := y; i < h; i += n {
		canvas.Line(x, i, h, i)
	}
	if !label {
		canvas.Gend()
		return
	}
	canvas.Gend()
	canvas.Gstyle("stroke:black")
	canvas.Line(0, y, 0, h)
	canvas.Line(x, 0, w, 0)
	canvas.Gend()

	canvas.Gstyle("text-anchor:middle;font-size:8;fill:white;stroke:black")
	for i := x; i < w; i += n {
		canvas.Circle(i, y, 2, "fill:black")
		canvas.Text(i, y-5, fmt.Sprintf("% 3v", i))
	}
	for i := x; i < h; i += n {
		canvas.Circle(x, i, 2, "fill:black")
		canvas.Text(x-10, i+2, fmt.Sprintf("% 3v", i))
	}
	canvas.Gend()
}

func DrawRegion(canvas *svg.SVG, r *region.Region) {
	canvas.Gid("region")
	spts := r.SentinalPoints()
	recColor := canvas.RGBA(10, 10, 10, 0.3)
	//canvas.Rect(5, 5, 90, 90, "fill:"+recColor+";stroke:"+recColor)
	min := r.Min()
	max := r.Max()
	canvas.Rect(int(min.X), int(min.Y), int(max.X-min.X), int(max.Y-min.Y), "stroke-dasharray:5,5;fill:none"+fmt.Sprintf(";stroke:rgb(%v,%v,%v)", 0, 200, 0))
	for _, pt := range spts {
		DrawPoint(canvas, int(pt.X), int(pt.Y), recColor)
	}
	canvas.Gend()
}

func DrawPolygon(canvas *svg.SVG, p tegola.Polygon, id string, style string, pointStyle string) {
	var points []maths.Pt
	canvas.Gid(id)
	canvas.Gid("path")
	path := ""
	for _, l := range p.Sublines() {
		pts := l.Subpoints()
		if len(pts) == 0 {
			continue
		}
		for i, pt := range pts {
			points = append(points, maths.Pt{X: pt.X(), Y: pt.Y()})
			if i == 0 {
				path += "M "
			} else {
				path += "L "
			}
			path += fmt.Sprintf("%v %v ", pt.X(), pt.Y())
		}

		path += "Z "
	}
	canvas.Path(path, style)
	canvas.Gend()
	canvas.Gid("Points")
	for _, pt := range points {
		x, y := int(pt.X), int(pt.Y)
		if pointStyle == "" {
			pointStyle = "fill:black"
		}
		canvas.Circle(x, y, 1, pointStyle)
		canvas.Group(fmt.Sprintf(`id="pt%v_%v" style="text-anchor:middle;font-size:8;fill:white;stroke:black;opacity:0"`, x, y))
		canvas.Text(x, y+5, fmt.Sprintf("(%v %v)", x, y))
		canvas.Gend()
	}
	canvas.Gend()
	canvas.Gend()
}

func DrawLine(canvas *svg.SVG, l tegola.LineString, id string, style string, pointStyle string) {
	var points []maths.Pt
	pts := l.Subpoints()

	canvas.Gid(id)
	path := ""
	canvas.Gid(id + "_path")
	for i, pt := range pts {
		points = append(points, maths.Pt{X: pt.X(), Y: pt.Y()})
		if i == 0 {
			path += "M "
		} else {
			path += "L "
		}
		path += fmt.Sprintf("%v %v ", pt.X(), pt.Y())
	}
	canvas.Path(path, style)
	canvas.Gend()
	canvas.Gid(id + "_points")
	for _, pt := range points {
		x, y := int(pt.X), int(pt.Y)
		if pointStyle == "" {
			pointStyle = "fill:black"
		}
		canvas.Circle(x, y, 1, pointStyle)
		canvas.Group(fmt.Sprintf(`id="pt%v_%v" style="text-anchor:middle;font-size:8;fill:white;stroke:black;opacity:0"`, x, y))
		canvas.Text(x, y+5, fmt.Sprintf("(%v %v)", x, y))
		canvas.Gend()
	}
	canvas.Gend()
	canvas.Gend()
}

func NewCanvas(writer io.Writer, w, h int, minx, miny, maxx, maxy int) *svg.SVG {
	canvas := svg.New(writer)
	canvas.Startview(w, h, minx-20, miny-20, maxx-(minx-20), maxy-(miny-20))
	canvas.Gid("grid")
	DrawGrid(canvas, minx, miny, maxx-minx, maxy-miny, 10, false, "stroke:gray")
	DrawGrid(canvas, minx, miny, maxx-minx, maxy-miny, 100, true, "stroke:black")
	canvas.Gend()
	return canvas
}

func MinMaxForPolygon(p tegola.Polygon) (minx, miny, maxx, maxy int) {
	for _, l := range p.Sublines() {
		for _, pt := range l.Subpoints() {
			x, y := int(pt.X()), int(pt.Y())
			minx, miny, maxx, maxy = MinMax(minx, miny, maxx, maxy, x, y, x, y)
		}
	}
	return
}

func MinMaxForPolygonSlice(ps []tegola.Polygon) (minx, miny, maxx, maxy int) {
	for _, p := range ps {
		mx, my, mmx, mmy := MinMaxForPolygon(p)
		minx, miny, maxx, maxy = MinMax(minx, miny, maxx, maxy, mx, my, mmx, mmy)
	}
	return
}

func MinMax(iminx, iminy, imaxx, imaxy, mx, my, mmx, mmy int) (minx, miny, maxx, maxy int) {
	minx, miny, maxx, maxy = iminx, iminy, imaxx, imaxy
	if mx < minx {
		minx = mx
	}
	if my < miny {
		miny = my
	}
	if mmx > maxx {
		maxx = mmx
	}
	if mmy > maxy {
		maxy = mmy
	}
	return
}
