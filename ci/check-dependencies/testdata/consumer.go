package consumer

import (
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	_ "github.com/alexeydott/tegola/provider/postgis"
)

// The public API must retain upstream geometry type identity.
var Bounds *geom.Extent = tegola.WebMercatorBounds
