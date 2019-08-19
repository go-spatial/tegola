package walker

import "fmt"

func cut(rng *[][2]float64, start, end int) (sliver [][2]float64) {
	if start < 0 || end < 0 || start >= len(*rng) || end > len(*rng) {
		panic(fmt.Sprintf("index out of bounds[0 - %v], start %v end %v", len(*rng), start, end))
	}
	switch {
	case end < start:

		l := (len(*rng) - start) + end
		sliver = make([][2]float64, l)

		copy(sliver, (*rng)[start:])
		copy(sliver[len((*rng)[start:]):], (*rng)[:end])

		copy(*rng, (*rng)[end:start])
		*rng = (*rng)[:start-end]

	case end == start:

		sliver = [][2]float64{(*rng)[end]}
		copy((*rng)[end:], (*rng)[end+1:])
		*rng = (*rng)[:len(*rng)-1]

	default:

		l := end - start
		sliver = make([][2]float64, l)
		copy(sliver, (*rng)[start:end])
		copy((*rng)[start:], (*rng)[end:])
		*rng = (*rng)[:len(*rng)-l]

	}
	return sliver
}
