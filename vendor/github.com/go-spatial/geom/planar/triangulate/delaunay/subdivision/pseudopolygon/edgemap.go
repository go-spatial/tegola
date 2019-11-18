package pseudopolygon

import (
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar"
)

type edgeMap map[geom.Line]bool

func (em edgeMap) Contains(p1, p2 geom.Point) bool {
	ln := geom.Line{p1, p2}
	normalizeLine(&ln)
	return em[ln]
}
func (em edgeMap) AddEdge(ln geom.Line) {
	normalizeLine(&ln)
	em[ln] = true
}

func (em edgeMap) Edges() (lns []geom.Line) {
	lns = make([]geom.Line, 0, len(em))
	for ln := range em {
		lns = append(lns, ln)
	}
	sort.Sort(sort.Reverse(planar.LinesByLength(lns)))
	return lns
}

// normalizeLine will order the line so that it's faces counter-clockwise when yPositive is downward
func normalizeLine(ln *geom.Line) {
	if cmp.PointLess(ln[0], ln[1]) {
		ln[0], ln[1] = ln[1], ln[0]
	}
}

func newEdgeMap(points []geom.Point) edgeMap {

	em := make(edgeMap)
	lp := len(points) - 1
	for i := range points {
		em.AddEdge(geom.Line{points[lp], points[i]})
		lp = i
	}
	return em
}
