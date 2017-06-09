package list

import (
	"fmt"
	"log"
)

type Elementer interface {
	// Prev returns the previous list element or nil.
	Prev() Elementer
	// Next returns the next list element or nil
	Next() Elementer
	// SetNext will set this list element next element to the provided element, and return the element or nil that was point to before.
	// SetNext does not call SetPrev.
	SetNext(n Elementer) Elementer
	// SetPrev will set this list element's prev element to the provided element, if
	// SetPrev does not call SetNext.
	SetPrev(n Elementer) Elementer

	// List is the list this Node belongs to.
	List() *List
	// SetList set the list this node should belong to; should return the old list or nil if it does not belong to a list.
	SetList(l *List) *List
}

type Sentinel struct {
	next Elementer
	prev Elementer
	list *List
}

func (s *Sentinel) SetNext(e Elementer) (oldElement Elementer) {
	oldElement = s.next
	s.next = e
	return oldElement
}

func (s *Sentinel) SetPrev(e Elementer) (oldElement Elementer) {
	oldElement = s.prev
	s.prev = e
	return oldElement
}
func (s *Sentinel) Next() Elementer {
	if p := s.next; s.list != nil && p != s.list.root {
		return p
	}
	return nil
}
func (s *Sentinel) Prev() Elementer {
	if p := s.prev; s.list != nil && p != s.list.root {
		return p
	}
	return nil
}

func (s *Sentinel) String() string {
	return fmt.Sprintf("Sentinel(%p)[list: %p][n:%p,p:%p]", s, s.list, s.next, s.prev)
}

func (s *Sentinel) List() *List {
	return s.list
}
func (s *Sentinel) SetList(l *List) (oldList *List) {
	oldList = s.list
	s.list = l
	return oldList
}

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

type List struct {
	root Elementer // sentinel list element, only &root, root.Prev(), and root.Next() are used
	len  int       // current list length excluding (this) sentinel element
}

// Init initializes or clears list l.
func (l *List) Init() *List {
	s := &Sentinel{}
	s.SetList(l)
	l.root = s
	l.len = 0
	return l
}

// lazyInit lazily initializes a zero List value
func (l *List) lazyInit() {
	if l.root == nil {
		l.Init()
	}
}

// New returns an initialized list.
func New() *List { return new(List).Init() }

// Len returns the number of elements of list l
func (l *List) Len() int { return l.len }

// Front returns the first element of list l or nil.
func (l *List) Front() Elementer {
	if l.len == 0 {
		return nil
	}
	return l.root.Next()
}

// Back returns the last element of list l or nil.
func (l *List) Back() Elementer {
	if l.len == 0 {
		return nil
	}
	return l.root.Prev()
}

// insert inserts e after at, incements l.len, and returns e
func (l *List) insert(e Elementer, at Elementer) Elementer {
	if e == nil {
		return e
	}
	root := l.root
	n := at.Next()
	// This means that n is at the end of the chain.
	if n == nil {
		n = root
	}

	at.SetNext(e)

	e.SetPrev(at)
	e.SetNext(n)

	n.SetPrev(e)
	e.SetList(l)
	//log.Printf("Element(%v - %[1]p) inserted into list(%p), it's list is (%p)\n", e, l, e.List())

	l.len++
	return e
}

// remove removes e from it's list, decrements l.len and returns e.
func (l *List) remove(e Elementer) Elementer {
	p := e.Prev()
	// Need to check to see if list.root.Next() == e
	if p == nil && l.root.Next() == e {
		p = l.root
	}
	n := e.Next()
	if n == nil && l.root.Prev() == e {
		n = l.root
	}

	if p != nil {
		p.SetNext(n)
	}
	if n != nil {
		n.SetPrev(p)
	}
	e.SetNext(nil) // Let the element know that they don't have access to those Elementers.
	e.SetPrev(nil) // Let the element know that they don't have access to those Elementers.
	e.SetList(nil)
	l.len--
	return e
}

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
	l.lazyInit()
	return l.insert(e, l.root)
}

// PushBack inserts a new element e to the back of the list l and returns e.
func (l *List) PushBack(e Elementer) Elementer {
	l.lazyInit()
	p := l.root.Prev()
	if p == nil {
		p = l.root
	}
	return l.insert(e, p)
}

// InsertBefore inserts a new element e immediately before mark and returns e.
// If mark is not an element of , the list is not modified.
func (l *List) InsertBefore(e Elementer, mark Elementer) Elementer {
	if mark.List() != l {
		log.Println("List don't match.")
		return nil
	}
	// see comment in List.Remove about initialization of l
	p := mark.Prev()
	if p == nil {
		log.Println("Using root for previous.")
		p = l.root
	}
	return l.insert(e, p)
}

// InsertAfter inserts a new element e immediately after the mark and returns e.
// If mark is not an element of l, the list is not modified.
func (l *List) InsertAfter(e Elementer, mark Elementer) Elementer {
	if mark.List() != l {
		return nil
	}
	// see comment in List.Remove about initialization of l
	return l.insert(e, mark)
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
func (l *List) MoveToFront(e Elementer) {
	if e.List() != l || l.root.Next() == e {
		return
	}
	l.insert(l.remove(e), l.root)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
func (l *List) MoveToBack(e Elementer) {
	if e.List() != l || l.root.Prev() == e {
		return
	}
	p := l.root.Prev()
	if p == nil {
		p = l.root
	}
	l.insert(l.remove(e), p)
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modifed
func (l *List) MoveBefore(e, mark Elementer) {
	if e.List() != l || e == mark || mark.List() != l {
		return
	}
	p := mark.Prev()
	if p == nil {
		p = l.root
	}
	// Are the place we want to change and the it the same?
	if p == e {
		return
	}
	l.insert(l.remove(e), p)
}

// MoveAfter move element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
func (l *List) MoveAfter(e, mark Elementer) {
	if e.List() != l || e == mark || mark.List() != l {
		return
	}
	l.insert(l.remove(e), mark)
}

// Replace replaces the mark with the new element e, and returns the mark.
// If mark is not an element of l, the list is not modified and nil is returned.
func (l *List) Replace(e, mark Elementer) Elementer {
	if mark.List() != l {
		return nil
	}
	n := mark.Next()
	l.Remove(mark)
	l.InsertBefore(e, n)
	return mark
}

// FindElementForward will start at the start element working it's way to the end element (or back to the sentinel element of the list if nil is provided) calling the match function.
// It return that element or nil if it did not find a point.
// Both start and end have to be in the list otherwise the function will return nil.
func (l *List) FindElementForward(start, end Elementer, finder func(e Elementer) (didFind bool)) (found Elementer) {
	if l == nil || l.len == 0 {
		return nil
	}
	if start == nil {
		start = l.Front()
	}
	if end == nil {
		end = l.Back()
	}
	if start.List() != l || end.List() != l {
		return nil
	}
	sawNil := false
	for e := start; ; e = e.Next() {
		// If we reach the end of the list we need to double back
		// to the front.
		if e == nil {
			if sawNil {
				break
			}
			sawNil = true
			e = l.Front()
		}
		if finder(e) {
			return e
		}
		if e == end {
			break
		}
	}
	return nil
}

// FindElementBackward will start at the start elemente working it's way backwards to the end element (or back to the sentinel element of the list if nil is provided) calling the match function.
// It returns the element that was found or nil if it did not find any element.
// Both start and end have to be in the list otherwise nil is retured.
func (l *List) FindElementBackward(start, end Elementer, finder func(e Elementer) (didFind bool)) (found Elementer) {
	if l == nil || l.len == 0 {
		return nil
	}
	if l == nil || l.len == 0 {
		return nil
	}
	if start == nil {
		start = l.Back()
	}
	if end == nil {
		end = l.Front()
	}
	if start.List() != l || end.List() != l {
		return nil
	}

	for e := start; e != end.Prev(); e = e.Prev() {
		if finder(e) {
			return e
		}
	}
	return nil
}

// IsSentinel returns weather or not the provided element is the sentinel element of the list.
func (l *List) IsSentinel(e Elementer) bool { return e.List() == l && e == l.root }
