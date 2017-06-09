package svg

import (
	"encoding/xml"
	"fmt"
	"io"

	"log"

	svg "github.com/ajstarks/svgo"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths"
)

const DefaultSpacing = 10

type Canvas struct {
	*svg.SVG
	Board  MinMax
	Region MinMax
}

func (canvas *Canvas) DrawPoint(x, y int, fill string) {
	canvas.Gstyle("text-anchor:middle;font-size:8;fill:white;stroke:black")
	canvas.Circle(x, y, 1, "fill:"+fill)
	canvas.Text(x, y+5, fmt.Sprintf("(%v %v)", x, y))
	canvas.Gend()
}

func drawGrid(canvas *Canvas, mm *MinMax, n int, label bool, id, style, pointstyle, ostyle string) {
	x, y, w, h := int(mm.MinX), int(mm.MinY), int(mm.Width()), int(mm.Height())
	canvas.Group(fmt.Sprintf(`id="%v"`, id), fmt.Sprintf(`style="%v"`, style))
	// Draw all the horizontal and vertical lines.
	for i := x; i < x+w; i += n {
		canvas.Line(i, y, i, y+h)
	}
	for i := y; i < y+h; i += n {
		canvas.Line(x, i, x+w, i)
	}
	if !label {
		canvas.Gend()
		return
	}
	canvas.Gend()
	canvas.Group(fmt.Sprintf(`id="%v"`, id+"_origin"), fmt.Sprintf(`style="%v"`, ostyle))
	canvas.Line(0, y, 0, h)
	canvas.Line(x, 0, w, 0)
	canvas.Gend()
	canvas.Group(fmt.Sprintf(`id="%v"`, id+"_points"), fmt.Sprintf(`style="%v"`, pointstyle))
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

func (canvas *Canvas) Comment(s string) *Canvas {
	fmt.Fprint(canvas.Writer, "<!-- \n")
	xml.Escape(canvas.Writer, []byte(s))
	fmt.Fprint(canvas.Writer, "\n -->")
	return canvas
}
func (canvas *Canvas) Commentf(format string, a ...interface{}) *Canvas {
	fmt.Fprint(canvas.Writer, "<!-- \n")
	xml.Escape(canvas.Writer, []byte(fmt.Sprintf(format, a...)))
	fmt.Fprint(canvas.Writer, "\n -->")
	return canvas
}

func (canvas *Canvas) DrawGrid(n int, label bool, style string) {
	drawGrid(canvas, &canvas.Board, n, label, fmt.Sprintf("board_%v", n), style, "text-anchor:middle;font-size:8;fill:white;stroke:black", "stroke:black")
}

func (canvas *Canvas) DrawRegion(withGrid bool) {

	canvas.Group(`id="region"`, `style="opacity:0.2"`)
	//	recColor := canvas.RGBA(10, 10, 10, 0.3)
	//canvas.Rect(5, 5, 90, 90, "fill:"+recColor+";stroke:"+recColor)
	canvas.Rect(int(canvas.Region.MinX), int(canvas.Region.MinY), int(canvas.Region.Width()), int(canvas.Region.Height()), "stroke-dasharray:5,5;fill:red;opacity:0.3;"+fmt.Sprintf(";stroke:rgb(%v,%v,%v)", 0, 200, 0))

	if withGrid {
		drawGrid(canvas, &canvas.Region, 10, false, "region_10", "stroke:red;opacity:0.2", "text-anchor:middle;font-size:8;fill:white;stroke:red", "stroke:red")
		drawGrid(canvas, &canvas.Region, 100, true, "region_100", "stroke:red;opacity:0.3", "text-anchor:middle;font-size:8;fill:white;stroke:red", "stroke:red")
	}
	/*
		for _, pt := range canvas.Region.SentinalPts() {
			canvas.DrawPoint(int(pt[0]), int(pt[1]), recColor)
		}
	*/
	canvas.Gend()
}

func (canvas *Canvas) DrawPolygon(p tegola.Polygon, id string, style string, pointStyle string, drawPoints bool) {
	var points []maths.Pt
	canvas.Group(`id="`+id+`"`, `style="opacity:1"`)
	canvas.Gid("polygon_path")
	path := ""
	pointCount := 0
	for _, l := range p.Sublines() {
		pts := l.Subpoints()
		if len(pts) == 0 {
			continue
		}
		idx := len(pts)
		// If the first and last point is the same skipp the last point.
		if pts[0].X() == pts[idx-1].X() && pts[0].Y() == pts[idx-1].Y() {
			idx = len(pts) - 1
		}
		if idx <= 0 {
			continue
		}
		for i, pt := range pts[:idx] {
			points = append(points, maths.Pt{X: pt.X(), Y: pt.Y()})
			if i == 0 {
				path += "M "
			} else {
				path += "L "
			}
			path += fmt.Sprintf("%v %v ", pt.X(), pt.Y())
			pointCount++
		}

		path += "Z "
	}
	canvas.Commentf("Point Count: %v", pointCount)
	log.Println("PointCount: ", pointCount)
	canvas.Path(path, fmt.Sprintf(`id="%v_%v"`, id, pointCount), style)
	canvas.Gend()
	/*
		if pointStyle == "" {
			pointStyle = "fill:black"
		}
		if drawPoints {
			canvas.Gid("Points")
			for _, pt := range points {
				x, y := int(pt.X), int(pt.Y)

				canvas.Circle(x, y, 1, pointStyle)
				//canvas.Group(fmt.Sprintf(`id="pt%v_%v" style="text-anchor:middle;font-size:8;fill:white;stroke:black;opacity:0"`, x, y))
				//canvas.Text(x, y+5, fmt.Sprintf("(%v %v)", x, y))
				//canvas.Gend()
			}
			canvas.Gend()
		}
	*/
	canvas.Gend()
}

func (canvas *Canvas) DrawMultiPolygon(mp tegola.MultiPolygon, id string, style string, pointStyle string, drawPoints bool) {
	canvas.Gid(id)
	for i, p := range mp.Polygons() {
		canvas.DrawPolygon(p, fmt.Sprintf("%v_mp_%v", id, i), style, pointStyle, drawPoints)
	}
	canvas.Gend()
}

func (canvas *Canvas) DrawLine(l tegola.LineString, id string, style string, pointStyle string, drawPoints bool) {
	var points []maths.Pt
	pts := l.Subpoints()

	canvas.Gid(id)
	path := ""
	canvas.Gid(id + "_line_path")
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
	if drawPoints {
		canvas.Gid(id + "_points")
		for _, pt := range points {
			x, y := int(pt.X), int(pt.Y)
			if pointStyle == "" {
				pointStyle = "fill:black"
			}
			canvas.Circle(x, y, 1, pointStyle)
			canvas.Group(fmt.Sprintf(`id="pt%v_%v" style="text-anchor:middle;font-size:8;fill:white;stroke:black;opacity:0.7"`, x, y))
			canvas.Text(x, y+5, fmt.Sprintf("(%v %v)", x, y))
			canvas.Gend()
		}
		canvas.Gend()
	}
	canvas.Gend()
}

func (canvas *Canvas) DrawMultiLine(ml tegola.MultiLine, id string, style string, pointStyle string, drawPoints bool) {
	canvas.Gid(id)
	for i, l := range ml.Lines() {
		canvas.DrawLine(l, fmt.Sprintf("%v_%v", id, i), style, pointStyle, drawPoints)
	}
	canvas.Gend()
}

func (canvas *Canvas) DrawGeometry(geo tegola.Geometry, id string, style string, pointStyle string, drawPoints bool) {
	switch g := geo.(type) {
	case tegola.MultiLine:
		canvas.DrawMultiLine(g, "multiline_"+id, style, pointStyle, drawPoints)
	case tegola.MultiPolygon:
		canvas.DrawMultiPolygon(g, "multipolygon_"+id, style, pointStyle, drawPoints)
	case tegola.Polygon:
		canvas.DrawPolygon(g, "polygon_"+id, style, pointStyle, drawPoints)
	case tegola.LineString:
		canvas.DrawLine(g, "line_"+id, style, pointStyle, drawPoints)
	case tegola.Point:
		canvas.Gid("point_" + id)
		canvas.DrawPoint(int(g.X()), int(g.Y()), pointStyle)
		canvas.Gend()
	case tegola.MultiPoint:
		canvas.Gid("multipoint_" + id)
		for i, p := range g.Points() {
			canvas.Gid(fmt.Sprintf("mp_%v", i))
			canvas.DrawPoint(int(p.X()), int(p.Y()), pointStyle)
			canvas.Gend()
		}
		canvas.Gend()
	}
}

func (canvas *Canvas) Init(writer io.Writer, w, h int, grid bool) *Canvas {
	if canvas == nil {
		panic("Canvas can not be nil!")
	}
	canvas.SVG = svg.New(writer)

	canvas.Startview(w, h, int(canvas.Board.MinX-20), int(canvas.Board.MinY-20), int(canvas.Board.MaxX+20), int(canvas.Board.MaxY+20))
	if grid {
		canvas.GroupFn([]string{
			`id="grid"`,
		}, func(canvas *Canvas) {
			canvas.DrawGrid(10, false, "stroke:gray")
			canvas.DrawGrid(100, true, "stroke:black")
		},
		)

	}
	return canvas
}

func (canvas *Canvas) GroupFn(attr []string, fn func(c *Canvas)) {
	canvas.SVG.Group(attr...)
	fn(canvas)
	canvas.SVG.Gend()
}
