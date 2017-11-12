package basic

type Geometry interface {
	basicType() // does nothing, but there to make collection only work with basic types.
	String() string
}

// Collection type can represent one or more other basic types.
type Collection []Geometry

//Geometeries return a set of geometeies that make that collection.
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
