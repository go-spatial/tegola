package subject

import (
	"fmt"

	colour "github.com/logrusorgru/aurora"
	"github.com/terranodo/tegola/container/list/point/list"
	"github.com/terranodo/tegola/maths"
)

func (s *Subject) GoString() string {
	str := fmt.Sprintf("  Subject:(%v)", s.winding)
	for i, p := 0, s.List.Front(); p != nil; i, p = i+1, p.Next() {
		str += fmt.Sprintf("[%v](%#v)", i, colour.Green(p))
	}

	return str
}

func (s *Subject) DebugStringAugmented(augmenter func(idx int, e maths.Pt) string) string {
	str := fmt.Sprintf("  Subject:(%v)", s.winding)
	for i, p := 0, s.List.Front(); p != nil; i, p = i+1, p.Next() {
		pt, ok := p.(list.ElementerPointer)
		if !ok {
			continue
		}
		str += augmenter(i, pt.Point())
	}
	return str
}
