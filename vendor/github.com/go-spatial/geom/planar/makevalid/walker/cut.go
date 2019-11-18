package walker

import "fmt"

// removeBridge is a quick hack to remove bridges that are getting left over
func removeBridge(rng [][2]float64) [][2]float64 {
	nrng := make([][2]float64, 0, len(rng))
	addLst := true
	for li, i := len(rng)-1, 0; i < len(rng)-1; {
		if rng[li] == rng[i+1] {
			li, i = i+1, i+2
			addLst = i != len(rng)
			continue
		}
		nrng = append(nrng, rng[i])
		li, i = i, i+1
	}
	if addLst {
		nrng = append(nrng, rng[len(rng)-1])
	}
	return nrng
}
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
