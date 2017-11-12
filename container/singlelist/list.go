package list

import "context"

// Elementer is a node in the list. Manages it's next and previous elements.
type Elementer interface {
	// Next returns the next list element or nil
	Next() Elementer
	// SetNext will set this list element next element to the provided element, and return the element or nil that was point to before.
	// SetNext does not call SetPrev.
	SetNext(n Elementer) Elementer

	// List is the list this Node belongs to.
	List() *List
	// SetList set the list this node should belong to; should return the old list or nil if it does not belong to a list.
	SetList(l *List) *List
}

// List contains pointers to the start element and the end element of a list.
type List struct {
	root Elementer // sentinel list element, only &root, root.Prev(), and root.Next() are used
	end  Elementer
	len  int // current list length excluding (this) sentinel element
}

// New returns an initialized list.
func New() *List { return new(List) }

// Len returns the number of elements of list l
func (l *List) Len() int { return l.len }

// Front returns the first element of list l or nil.
func (l *List) Front() Elementer {
	if l == nil || l.len == 0 {
		return nil
	}
	return l.root
}

// Back returns the last element of the list l or nil.
func (l *List) Back() Elementer {
	if l == nil || l.len == 0 {
		return nil
	}
	return l.end
}

// IsInList will check  if the element is in the list.
func (l *List) IsInList(e Elementer) bool {
	if e.List() != l {
		return false
	}
	return l.GetBefore(e) != nil
}

// insert inserts e after at, incements l.len, and returns e
func (l *List) insert(e Elementer, at Elementer) Elementer {

	if e == nil || at == nil || !l.IsInList(at) {
		return e
	}

	if l.root == nil {
		e.SetNext(e)
		l.root = e
		l.end = e
		l.len++
		return e
	}

	n := at.Next()
	at.SetNext(e)
	e.SetNext(n)
	e.SetList(l)
	//log.Printf("Element(%v - %[1]p) inserted into list(%p), it's list is (%p)\n", e, l, e.List())
	l.len++
	return e
}

// GetBefore will return the element before this element. If the element is not in the list, it will return nil.
func (l *List) GetBefore(m Elementer) Elementer {

	if m.List() != l {
		return nil
	}

	if l.Len() == 0 {
		return nil
	}

	if m == l.root {
		return l.end
	}

	last := l.root
	for e := l.root.Next(); e != l.root; e = e.Next() {
		if e == m {
			return last
		}
		last = e
	}
	return nil
}

// remove removes e from it's list, decrements l.len and returns e.
func (l *List) remove(e Elementer) Elementer {

	if l.root == e {
		r := l.root
		if r.Next() == l.root {

			l.root = nil
			l.len = 0
			l.end = nil
			return e

		}
		l.root = r.Next()
		l.end.SetNext(l.root)

		l.len--
		r.SetList(nil)
		r.SetNext(nil)
		return e
	}

	p := l.GetBefore(e)
	// e is not in the list.
	if p == nil {
		return e
	}
	n := e.Next()
	p.SetNext(n)
	if e == l.end {
		l.end = p
	}
	l.len--
	e.SetNext(nil) // Let the element know that they don't have access to those Elementers.
	e.SetList(nil)

	return e
}

// Remove will remove the element from this list if exists in the list.
func (l *List) Remove(e Elementer) Elementer {
	if e.List() == l {
		// if e.list == l, l must have been initialized when e was inserted in l or
		// l == nil (e is a zero Element) and l.remove will crash
		l.remove(e)
	}
	return e
}

// PushFront inserts a new element e to the front of the list l and returns e.
func (l *List) PushFront(e Elementer) Elementer {

	if e == nil {
		return e
	}

	if l.Len() == 0 {
		e.SetList(l)
		e.SetNext(e)
		l.end = e
		l.root = e
		l.len++
		return e
	}

	// Need to check to see if the element is already in the list.
	if e.List() == l {
		// Don't need to do anything as it's already at the start of the list.
		if e == l.root {
			return e
		}
		l.remove(e)
	}
	e.SetNext(l.root)
	e.SetList(l)
	l.end.SetNext(e)
	l.root = e
	l.len++
	return e

}

// PushBack inserts a new element e to the back of the list l and returns e.
func (l *List) PushBack(e Elementer) Elementer {
	if e == nil {
		return e
	}

	if l.Len() == 0 {
		e.SetList(l)
		e.SetNext(e)
		l.end = e
		l.root = e
		l.len++
		return e
	}

	// If element is already in the list, we need to move it to the back.
	if e.List() == l {
		// It's already in the back. No need to do anything.
		if e == l.end {
			return e
		}
		l.remove(e)
	}
	e.SetNext(l.root)
	e.SetList(l)
	l.end.SetNext(e)
	l.end = e
	l.len++
	return e

}

// InsertBefore inserts a new element e immediately before mark and returns e.
// If mark is not an element of , the list is not modified.
func (l *List) InsertBefore(e Elementer, mark Elementer) Elementer {
	if !l.IsInList(mark) {
		return nil
	}

	if mark == l.root {
		return l.PushFront(e)

	}

	p := l.GetBefore(mark)
	if p == nil {
		return p
	}

	return l.insert(e, p)
}

// InsertAfter inserts a new element e immediately after the mark and returns e.
// If mark is not an element of l, the list is not modified.
func (l *List) InsertAfter(e Elementer, mark Elementer) Elementer {
	if !l.IsInList(mark) {
		return nil
	}
	if mark == l.end {
		return l.PushBack(e)
	}

	return l.insert(e, mark)
}

// ForElementsBetween will start at the start element working it's way to the end element (or back to the sentinel element of the list if nil is provided) calling the match function.
// It return that element or nil if it did not find a point.
// Both start and end have to be in the list otherwise the function will return nil.
func (l *List) FindElementsBetween(start, end Elementer, finder func(e Elementer) (didFind bool)) (found Elementer) {
	if l == nil || l.len == 0 {
		return nil
	}
	if start == nil {
		start = l.root
	}
	if end == nil {
		end = l.end
	}

	if start.List() != l || end.List() != l {
		return nil
	}

	for e := start; ; e = e.Next() {
		// If we reach the end of the list we need to double back
		// to the front.
		if finder(e) {
			return e
		}
		if e == end {
			return nil
		}
	}
	return nil
}

// ForEach call fn for each element in the list. Return false stops the iteration.
func (l *List) ForEach(fn func(e Elementer) bool) {
	if l == nil || l.len == 0 {
		return
	}

	for e := l.root; ; e = e.Next() {
		if !fn(e) || e == l.end {
			break
		}
	}
}

// Range will start up a go routine, and send every element in the list at the time the call is made. It will make a copy
// of the elements in the list, so that changes in the list does not effect order.
// The channel is closed and the go routine exists when the list is exhausted or the ctx is done.
func (l *List) Range(ctx context.Context) <-chan Elementer {
	c := make(chan Elementer)

	go func() {
		// keep a copy so that changes to the list don't effect the iterations.
		var els []Elementer
		for e := l.root; ; e = e.Next() {
			els = append(els, e)
			if e == l.end {
				break
			}
		}
		for i := range els {
			select {
			case c <- els[i]:
			case <-ctx.Done():
				return
			}
		}
		close(c)
	}()
	return c
}

// ForEach call fn for each element in the list. Return false stops the iteration.
func (l *List) ForEachIdx(fn func(idx int, e Elementer) bool) {
	if l == nil || l.len == 0 {
		return
	}

	idx := 0
	for e := l.root; ; e = e.Next() {
		if !fn(idx, e) || e == l.end {
			break
		}
		idx++
	}
}

// Clear will remove all the items from the list.
func (l *List) Clear() {
	if l == nil || l.len == 0 {
		return
	}

	l.end.SetNext(nil)
	for e := l.root; e != nil; {
		c := e
		e = e.Next()
		c.SetList(nil)
		c.SetNext(nil)
	}
	l.len = 0
	l.root = nil
	l.end = nil

}
