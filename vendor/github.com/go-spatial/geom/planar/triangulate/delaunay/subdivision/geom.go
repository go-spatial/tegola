package subdivision

import (
	"context"
	"log"
	"strings"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/quadedge"
	"github.com/go-spatial/geom/winding"
)

// AsGeom returns a geom based Triangle
func (t Triangle) AsGeom() (tri geom.Triangle) {
	e := t.StartingEdge()
	for i := 0; i < 3; e, i = e.RNext(), i+1 {
		tri[i] = [2]float64(*e.Orig())
	}
	return tri
}

// NewSubdivisionFromGeomLines returns a new subdivision made up of the given geom lines.
// it is assume that all line are connected. If lines are disjointed that it is undefined
// which disjointed subdivision will be returned
func NewSubdivisionFromGeomLines(lines []geom.Line, order winding.Order) *Subdivision {
	lines = planar.NormalizeUniqueLines(lines)

	var (
		indexMap = make(map[geom.Point]*quadedge.Edge)
		ext      *geom.Extent

		eq *quadedge.Edge
		oe *quadedge.Edge
		de *quadedge.Edge
	)

	for i := range lines {
		orig, dest := geom.Point(lines[i][0]), geom.Point(lines[i][1])
		if geom.IsEmpty(orig) || geom.IsEmpty(dest) {
			log.Printf("orig %v or dest %v is empty", orig, dest)
		}
		if ext == nil {
			ext = geom.NewExtentFromPoints(orig, dest)
		} else {
			ext.AddPointers(orig, dest)
		}

		oe = indexMap[orig]
		de = indexMap[dest]

		if oe != nil {
			if oe.FindONextDest(dest) != nil {
				// edge already in graph
				continue
			}
			oe, _ = quadedge.ResolveEdge(oe, dest)
			if oe == nil {
				oe = indexMap[orig]
			}
		}

		if de != nil {
			de, _ = quadedge.ResolveEdge(de, orig)
			if de == nil {
				de = indexMap[dest]
			}
		}

		eq = quadedge.New()
		eq.EndPoints(&orig, &dest)

		switch {
		case oe != nil && de != nil:
			eq = quadedge.Connect(oe.Sym(), de, order)

		case oe != nil && de == nil:
			quadedge.Splice(oe, eq)
			indexMap[dest] = eq.Sym()

		case oe == nil && de != nil:
			quadedge.Splice(de, eq.Sym())
			indexMap[orig] = eq

		case oe == nil && de == nil:
			indexMap[orig] = eq
			indexMap[dest] = eq.Sym()

		}
	}

	tri, _ := geom.NewTriangleForExtent(ext, 0)
	sd := &Subdivision{
		frame: [3]geom.Point{
			geom.Point(tri[0]),
			geom.Point(tri[1]),
			geom.Point(tri[2]),
		},
		ptcount:      len(indexMap),
		startingEdge: eq,
	}
	if debug {
		var str strings.Builder
		DumpSubdivisionW(&str, sd)
		log.Print(str.String())
	}
	if err := sd.Validate(context.Background()); err != nil {
		err1 := err.(quadedge.ErrInvalid)
		for i, str := range err1 {
			log.Printf("%03v: %v", i, str)
		}
	}
	return sd
}
