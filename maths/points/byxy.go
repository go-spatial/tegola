package points

import (
	"sort"

	"github.com/terranodo/tegola/maths"
)

type ByXY []maths.Pt

func (t ByXY) Len() int           { return len(t) }
func (t ByXY) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t ByXY) Less(i, j int) bool { return maths.XYOrder(t[i], t[j]) == -1 }
func (t ByXY) Sort()              { sort.Sort(t) }
