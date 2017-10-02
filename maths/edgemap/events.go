package edgemap

import (
	"sort"

	"github.com/terranodo/tegola/maths"
)

type event struct {
	edge int
	pt   *maths.Pt
}
type byXY []event

func (a byXY) Len() int { return len(a) }
func (a byXY) Swap(i, j int) {
	v := a[i]
	a[i] = a[j]
	a[j] = v
}
func (a byXY) Less(i, j int) bool {
	if a[i].pt.X < a[j].pt.X {
		return true
	}
	return a[i].pt.X == a[j].pt.X && a[j].pt.Y < a[j].pt.Y
}

func (a byXY) Sort() { sort.Sort(a) }
