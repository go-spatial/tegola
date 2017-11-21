package points

import (
	"fmt"
	"math"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/util"
)

type BoundingBox [4]float64

func (bb BoundingBox) PointAt(i int) maths.Pt {
	if i >= 4 {
		i = i % 4
	}
	switch i {
	case 0:
		return maths.Pt{bb[0], bb[1]}
	case 1:
		return maths.Pt{bb[0], bb[3]}
	case 2:
		return maths.Pt{bb[2], bb[3]}
	default:
		return maths.Pt{bb[2], bb[1]}
	}
}

func (bb BoundingBox) ContainBB(bb1 [4]float64) bool {
	return bb[0] <= bb1[0] && // bb1 minx is bigger or the same as bb
		bb[1] <= bb1[1] && // bb1 miny is bigger or the same as bb
		bb[2] >= bb1[2] && // bb1 maxx is smaller or the same as bb
		bb[3] >= bb1[3] // bb1 maxy is smaller or the same as bb

}

func (bb BoundingBox) DisjointBB(bbox2 [4]float64) bool {
	// Returns true if the bounding boxes overlap, false otherwise.
	// The two bounding boxe values should be in the same projection & in the form:
	//	[MinX, MinY, MaxX, MaxY]
	disjoint := (bb[0] > bbox2[2] || bb[1] > bbox2[3] || bb[2] < bbox2[0] || bb[3] < bbox2[1])
	return disjoint
}

func (bb BoundingBox) LREdges() [4]maths.Line {
	return [4]maths.Line{
		{maths.Pt{bb[0], bb[1]}, maths.Pt{bb[2], bb[1]}}, // MinX,MinY -> MaxX,MinY
		{maths.Pt{bb[0], bb[1]}, maths.Pt{bb[0], bb[3]}}, // MinX,MinY -> MinX,MaxY
		{maths.Pt{bb[0], bb[3]}, maths.Pt{bb[2], bb[3]}}, // MinX,MaxY -> MaxX,MaxY
		{maths.Pt{bb[2], bb[1]}, maths.Pt{bb[2], bb[3]}}, // MaxX,MinY -> MaxX,MaxX
	}
}

func (bb BoundingBox) Contains(pt maths.Pt) bool {
	return bb[0] <= pt.X && pt.X <= bb[2] &&
		bb[1] <= pt.Y && pt.Y <= bb[3]
}

func (bb BoundingBox) ContainsLine(l maths.Line) bool {
	return bb.Contains(l[0]) && bb.Contains(l[1])
}

func (bb BoundingBox) Area() float64 {
	return math.Abs((bb[2] - bb[0]) * (bb[3] - bb[1]))
}

// TODO:gdey — should we return an error?
func BBox(pts []maths.Pt) (bb [4]float64) {
	if len(pts) == 0 {
		return bb
	}
	bb = [4]float64{pts[0].X, pts[0].Y, pts[0].X, pts[0].Y}
	for _, pt := range pts[1:] {
		if pt.X < bb[0] {
			bb[0] = pt.X
		}
		if pt.Y < bb[1] {
			bb[1] = pt.Y
		}
		if pt.X > bb[2] {
			bb[2] = pt.X
		}
		if pt.Y > bb[3] {
			bb[3] = pt.Y
		}
	}
	return bb
}

func (bb BoundingBox) ConvertSrid(fromId int, toId int) BoundingBox {
	var convFunc func(int, tegola.Geometry) (basic.G, error)
	var convSrid int

	if fromId == tegola.WebMercator {
		convFunc = basic.FromWebMercator
		convSrid = toId
	} else if toId == tegola.WebMercator {
		convFunc = basic.ToWebMercator
		convSrid = fromId
	} else if fromId == toId {
		newBb := bb
		return newBb
	} else {
		util.CodeLogger.Fatalf("Converting from srid %v -> %v is currently unsupported\n", fromId, toId)
	}

	// Lower left & top right points
	ll := basic.Point{bb[0], bb[1]}
	tr := basic.Point{bb[2], bb[3]}

	// Same points converted
	llC, err1 := convFunc(convSrid, ll)
	trC, err2 := convFunc(convSrid, tr)

	if err1 != nil || err2 != nil {
		newBB := bb
		msg := "Problem converting BoundingBox geometry from %v -> %v: %v"
		util.CodeLogger.Error()
		if err1 != nil {
			util.CodeLogger.Errorf(msg, fromId, toId, err1)
		} else {
			util.CodeLogger.Errorf(msg, fromId, toId, err2)
		}
		return newBB
	}

	newBB := BoundingBox{llC.AsPoint().X(), llC.AsPoint().Y(), trC.AsPoint().X(), trC.AsPoint().Y()}

	return newBB
}

func (bb BoundingBox) AsGeoJSON() string {
	template := `
{
  "type": "Polygon",
  "coordinates": [
    [
      [%v, %v],
      [%v, %v],
      [%v, %v],
      [%v, %v],
      [%v, %v]
    ]
  ]
}
`
	geoJson := fmt.Sprintf(template, bb[0], bb[1], bb[2], bb[1], bb[2], bb[3], bb[0], bb[3], bb[0], bb[1])
	return geoJson
}
