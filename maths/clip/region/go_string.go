package region

import (
	"fmt"

	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

func (r *Region) GoString() string {
	str := fmt.Sprintf("   Region:(%v)", r.winding)
	for i, p := 0, r.List.Front(); p != nil; i, p = i+1, p.Next() {
		str += fmt.Sprintf("[%v](%#v)", i, p)
	}
	return str
}

func (r *Region) DebugStringAugmented(augmenter func(idx int, e maths.Pt) string) string {
	str := fmt.Sprintf("  Region:(%v)", r.winding)
	for i, p := 0, r.List.Front(); p != nil; i, p = i+1, p.Next() {
		pt, ok := p.(list.ElementerPointer)
		if !ok {
			continue
		}
		str += augmenter(i, pt.Point())
	}
	return str
}
