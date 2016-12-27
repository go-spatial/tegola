package intersect

import (
	"github.com/terranodo/tegola/container/list/point/list"
)

type Intersect struct {
	list.List
}

func New() *Intersect {
	l := new(Intersect)
	l.List.Init()
	return l
}
