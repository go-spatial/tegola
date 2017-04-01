package subject

import (
	"fmt"
)

func (s *Subject) GoString() string {
	str := fmt.Sprintf("  Subject:(%v)", s.winding)
	for i, p := 0, s.List.Front(); p != nil; i, p = i+1, p.Next() {
		str += fmt.Sprintf("[%v](%#v)", i, p)
	}
	return str
}
