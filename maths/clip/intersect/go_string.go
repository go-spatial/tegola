package intersect

import "fmt"

func (ib *Inbound) GoString() string {
	if ib == nil {
		return "(nil)"
	}
	if ib.pt == nil {
		return fmt.Sprintf("pt(nil)[%#v]", ib.seen)
	}
	return fmt.Sprintf("pt(%v)[%#v]", ib.pt, ib.seen)
}
