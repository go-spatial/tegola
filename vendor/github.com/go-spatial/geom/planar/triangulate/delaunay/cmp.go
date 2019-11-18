package delaunay

import (
	pkg "github.com/go-spatial/geom/cmp"
)

var cmp = pkg.HiCMP

var oldCmp = pkg.SetDefault(pkg.HiCMP)
