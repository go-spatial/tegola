package region

import (
	"fmt"
)

func (r *Region) GoString() string {
	str := fmt.Sprintf("   Region:(%v)", r.winding)
	for i, p := 0, r.List.Front(); p != nil; i, p = i+1, p.Next() {
		str += fmt.Sprintf("[%v](%#v)", i, p)
	}
	return str
}
