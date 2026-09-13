package basic

import "github.com/go-spatial/tegola"

type Geometry interface {
	basicType() // does nothing, but there to make collection only work with basic types.
	String() string
}

// Collection type can represent one or more other basic types.
type Collection []Geometry

// Geometries returns the set of geometries that make up this collection. This
// implements the tegola.Collection interface, which is required for callers
// such as internal/convert.ToGeom to be able to recognize and unwrap a
// basic.Collection. Without this method basic.Collection did not satisfy
// tegola.Collection (the method name/signature did not match), so any
// GEOMETRYCOLLECTION feature would fail to convert back to a geom.Geometry
// with an "Unknown Geometry" error, aborting the whole tile.
func (c Collection) Geometries() []tegola.Geometry {
	geometries := make([]tegola.Geometry, len(c))
	for i := range c {
		geometries[i] = c[i]
	}
	return geometries
}

//Geometeries return a set of geometeies that make that collection.
//
// Deprecated: kept only for backwards compatibility with any existing callers
// relying on the (misspelled) previous method name/signature; use
// Geometries instead, which satisfies tegola.Collection.
func (c Collection) Geometeries() (geometeries []G) {
	geometeries = make([]G, 0, len(c))
	for i := range c {
		geometeries = append(geometeries, G{c[i]})
	}
	return geometeries
}

func (Collection) String() string {
	return "Collection"
}

// private this is for membership to basic types.
func (Collection) basicType() {}
