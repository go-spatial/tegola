package points

import "github.com/terranodo/tegola/maths"

func Reverse(a []maths.Pt) []maths.Pt {
	l := len(a) - 1
	for i := 0; i < len(a) && (l-i) > i; i++ {
		a[i], a[l-i] = a[l-i], a[i]
	}
	return a
}
