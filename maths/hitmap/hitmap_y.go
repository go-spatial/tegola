package hitmap

import "github.com/terranodo/tegola/maths"

type segEventY struct {
	x1         int64 // float64
	y1         float64
	x2         int64 // float64
	y2         float64
	m          float64
	b          float64
	isMDefined bool
}

type segEventsY []segEventY

func (se segEventsY) Len() int { return len(se) }
func (se segEventsY) Less(i, j int) bool {
	if se[i].y1 == se[j].y1 {
		return se[i].x1 < se[j].x1
	}
	return se[i].y1 < se[j].y1
}
func (se segEventsY) Swap(i, j int) { se[i], se[j] = se[j], se[i] }
func (se *segEventsY) Add(l maths.Line) {
	if se == nil {
		return
	}
	// Skip dup points
	var ev segEventY
	if l[0].IsEqual(l[1]) {
		return
	}
	switch {
	case l[0].Y == l[1].Y && l[0].X > l[1].X:
		fallthrough
	case l[0].Y > l[1].Y:
		ev.x2 = int64(l[0].X * 100)
		ev.y2 = l[0].Y
		ev.x2 = int64(l[1].X * 100)
		ev.y2 = l[1].Y
	default:
		ev.x1 = int64(l[1].X * 100)
		ev.y1 = l[1].Y
		ev.x2 = int64(l[0].X * 100)
		ev.y2 = l[0].Y
	}
	ev.m, ev.b, ev.isMDefined = l.SlopeIntercept()
	*se = append(*se, ev)
}

func (se segEventsY) Contains(pt maths.Pt) bool {
	var i, count int
	var x, lx, rx int64
	var x100, x1100, x2100 int64
	for i = 0; i < len(se) && se[i].y1 <= pt.Y; i++ {
		x100 = int64(pt.X * 100)
		if se[i].x1 <= se[i].x2 {
			lx, rx = se[i].x1, se[i].x2
		} else {
			lx, rx = se[i].x2, se[i].x1
		}

		if x100 < lx || x100 > rx {
			continue
		}

		//y1100, y2100 = int64(seg.events[i].y1*100), int64(seg.events[i].y2*100)
		x1100, x2100 = se[i].x1, se[i].x2

		// Horizontal line
		if x1100 == x2100 &&
			x100 == x1100 {

			if se[i].y1 <= pt.Y &&
				pt.Y <= se[i].y2 {
				// if we are on the line return true.
				return true
			}
			continue
		}

		if x100 == x1100 && se[i].y1 < pt.Y {
			// We are going through a vertex.
			if x2100 <= x100 {
				count++
			}
			continue
		}
		if x100 == x2100 && se[i].y2 < pt.Y {
			// We are going through a vertex.
			if x1100 <= x100 {
				count++
			}
			continue
		}

		// the segment is verticle and the x is the same; the point is contained.
		if !se[i].isMDefined && pt.Y == se[i].y1 {
			return true
		}

		if pt.Y > se[i].y2 {
			count++
			continue
		}

		// need to solve for y.
		// y = mx + b
		// x = (b - y)/m
		x = int64(((se[i].b - pt.Y) / se[i].m) * 100)
		if x == x100 {
			return true
		}

		if (se[i].m < 1 && x < x100) ||
			(se[i].m > 0 && x > x100) {
			count++
			continue
		}
	}
	return count%2 != 0

}
