package list

import "fmt"

// Element allow one to use a list with a generic element.
type Element struct {
	Sentinel
	Value interface{}
}

func (e Element) String() string {
	return fmt.Sprintf("%v", e.Value)
}

func NewElement(v interface{}) *Element { return &Element{Value: v} }

func SliceOfElements(vals ...interface{}) []*Element {
	els := make([]*Element, 0, len(vals))
	for _, v := range vals {
		els = append(els, NewElement(v))
	}
	return els
}
