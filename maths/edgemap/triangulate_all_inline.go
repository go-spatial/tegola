package edgemap

import (
	"github.com/terranodo/tegola/maths"
)

func (em *EM) Triangulate() {
	//defer log.Println("Done with Triangulate")
	keys := em.Keys
	lnkeys := len(keys) - 1
	var lines []maths.Line

	//log.Println("Starting to Triangulate. Keys", len(keys))
	// We want to run through all the keys up to the last key, to generating possible edges, and then
	// collecting the ones that don't intersect with the edges in the map already.
	for i := 0; i < lnkeys; i++ {
		lookup := em.Map[keys[i]]
		var possibleEdges []maths.Line
		for j := i + 1; j < len(keys); j++ {
			if _, ok := lookup[keys[j]]; ok {
				// Already have an edge with this point
				continue
			}
			l := maths.Line{keys[i], keys[j]}
			possibleEdges = append(possibleEdges, l)
		}

		// Now we need to do a line sweep to see which of the possible edges we want to keep.
		lines = append([]maths.Line{}, possibleEdges...)
		offset := len(lines)
		lines = append(lines, em.Segments...)
		skiplines := make([]bool, offset)
		findIntersects(lines, skiplines)

		// Add the remaining possible Edges to the edgeMap.
		lines = lines[:0]
		for i := range possibleEdges {
			if skiplines[i] {
				continue
			}
			lines = append(lines, possibleEdges[i])
		}
		em.AddLine(false, true, false, lines...)
	}
}
