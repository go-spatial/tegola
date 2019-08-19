package subdivision

import (
	"errors"
	"fmt"
	"log"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/internal/debugger"
	"github.com/go-spatial/geom/planar/triangulate/gdey/quadedge/quadedge"
)

const (
	debug = false
)

// ErrAssumptionFailed is an assert of when our assumptions fails, in debug mode it will return and error. In
// non debug mode it will panic
func ErrAssumptionFailed() error {
	str := fmt.Sprintf("Assumption failed at: %v ", debugger.FFL(0))
	if debug {
		return errors.New(str)
	}
	panic(str)
}

// DumpSubdivision will print each edge in the subdivision
func DumpSubdivision(sd *Subdivision) {
	log.Printf("Frame: %#v\n", sd.frame)
	var edges geom.MultiLineString

	_ = sd.WalkAllEdges(func(e *quadedge.Edge) error {
		ln := e.AsLine()
		edges = append(edges, ln[:])
		return nil
	})
	log.Printf("Edges:\n%v", wkt.MustEncode(edges))
}
