package list

import (
	"fmt"
	"testing"
)

func checkListLen(t *testing.T, desc string, l *List, len int) bool {
	if n := l.Len(); n != len {
		t.Errorf("%v: l.Len() = %d, want %d", desc, n, len)
		return false
	}
	return true
}

func checkListPointers(t *testing.T, desc string, l *List, es []*Element) {
	root := l.root

	if !checkListLen(t, desc, l, len(es)) {
		return
	}

	// zero length lists must be the zero value or properly initialized (sentinel circle)
	if len(es) == 0 {
		if n, p := l.root.Next(), l.root.Prev(); (n != nil && n != root) ||
			(p != nil && p != root) {
			t.Errorf("%v: l.root.Next() = %p, l.root.Prev() = %p; both should be nil or %p", desc, n, p, root)
		}
		return
	}

	// len(es) > 0

	// check internal and external prev/ext connection
	var ok bool
	for i, e := range es {
		prev := root
		Prev := (Elementer)(nil)
		if i > 0 {
			prev = es[i-1]
			Prev, ok = prev.(Elementer)
			if !ok {
				t.Errorf("%s: Unable to convert es[%d-1](%T) to type *Element, ", desc, i, prev)
			}
		}

		if p := e.Prev(); p != Prev {
			t.Errorf("%s: elt[%d](%p).Prev() = %p, want %p", desc, i, e, p, Prev)
		}

		next := root
		Next := (Elementer)(nil)
		if i < len(es)-1 {
			next = es[i+1]
			Next, ok = next.(Elementer)
			if !ok {
				t.Errorf("%s: Unable to convert es[%d+1](%T) to type *Element, ", desc, i, next)
			}
		}
		if n := e.Next(); n != Next {
			t.Errorf("%s: elt[%d](%p).Next() = %p, want %p", desc, i, e, n, next)
		}
	}
}

func TestList(t *testing.T) {
	e := SliceOfElements("a", 1, 2, 3, "banana")
	l := New()
	checkListPointers(t, "Zero Element test on New List", l, []*Element{})

	// Single element list
	l.PushFront(e[0])
	checkListPointers(t, "One element test", l, e[0:1])
	l.MoveToFront(e[0])
	checkListPointers(t, "One element after Move to Front", l, e[0:1])
	l.MoveToBack(e[0])
	checkListPointers(t, "one element after Move to Back", l, e[0:1])
	l.Remove(e[0])
	checkListPointers(t, "zero element after Remove", l, []*Element{})

	// Bigger list
	l.PushFront(e[2])
	l.PushFront(e[1])
	l.PushBack(e[3])
	l.PushBack(e[4])

	checkListPointers(t, "4 element list", l, e[1:])

	l.Remove(e[2])
	checkListPointers(t, "3 element list after removing e2", l, []*Element{e[1], e[3], e[4]})

	l.MoveToFront(e[3]) // Move from the middle
	checkListPointers(t, "3 element move to front", l, []*Element{e[3], e[1], e[4]})

	l.MoveToFront(e[1])
	l.MoveToBack(e[3]) // Move from the middle
	checkListPointers(t, "3 element list 1,4,3", l, []*Element{e[1], e[4], e[3]})

	l.MoveToFront(e[3]) // Move from the back.
	checkListPointers(t, "3 element list move 3 from back", l, []*Element{e[3], e[1], e[4]})
	l.MoveToFront(e[3]) // Should be a no op.
	checkListPointers(t, "3 element list move no-op", l, []*Element{e[3], e[1], e[4]})

	l.MoveToBack(e[3]) // Move to the back.
	checkListPointers(t, "3 element list move 3 from back", l, []*Element{e[1], e[4], e[3]})
	l.MoveToBack(e[3]) // Should be a no op.
	checkListPointers(t, "3 element list move no-op", l, []*Element{e[1], e[4], e[3]})

	l.InsertBefore(e[2], e[1])
	checkListPointers(t, "4 element inserted e2 before e1", l, []*Element{e[2], e[1], e[4], e[3]})
	l.Remove(e[2])
	l.InsertBefore(e[2], e[4])
	checkListPointers(t, "4 element inserted e2 before e4", l, []*Element{e[1], e[2], e[4], e[3]})
	l.Remove(e[2])

	l.InsertAfter(e[2], e[1]) // insert after front
	checkListPointers(t, "4 element inserted e2 after e1", l, []*Element{e[1], e[2], e[4], e[3]})
	l.Remove(e[2])
	l.InsertAfter(e[2], e[4]) // insert after middle
	checkListPointers(t, "4 element inserted e2 after e4", l, []*Element{e[1], e[4], e[2], e[3]})
	l.Remove(e[2])
	l.InsertAfter(e[2], e[3]) // insert after back
	checkListPointers(t, "4 element inserted e2 after e3", l, []*Element{e[1], e[4], e[3], e[2]})
	l.Remove(e[2])

	// Check standard iteration.
	sum := 0
	for e := l.Front(); e != nil; e = e.Next() {
		if elem, ok := e.(*Element); ok {
			if i, ok := elem.Value.(int); ok {
				sum += i
			}
		}
	}
	if sum != 4 {
		t.Errorf("sum over l = %d, want 4", sum)
	}

	// Clear all elements by iterating
	var next Elementer
	for e := l.Front(); e != nil; e = next {
		next = e.Next()
		l.Remove(e)
	}
	checkListPointers(t, "Cleared list", l, []*Element{})

}

func checkList(t *testing.T, desc string, l *List, es []int) {
	if !checkListLen(t, desc, l, len(es)) {
		return
	}
	i := 0
	for el := l.Front(); el != nil; el = el.Next() {
		e, ok := el.(*Element)
		if !ok {
			t.Errorf("%s:elt[%d] is not of type Element.", desc, i)
			goto ContinueIter
		}
		{
			le := e.Value.(int)
			if le != es[i] {
				t.Errorf("%s:elt[%d].Value = %v, want %v", desc, i, le, es[i])
			}
		}
	ContinueIter:
		i++
	}
}

func TestRemove(t *testing.T) {
	l := New()
	e := SliceOfElements(1, 2)
	l.PushBack(e[0])
	l.PushBack(e[1])
	checkListPointers(t, "List with two items", l, e)
	ef := l.Front()
	l.Remove(ef)
	checkListPointers(t, "List with only e1", l, []*Element{e[1]})
	l.Remove(ef)
	checkListPointers(t, "Noop remove", l, []*Element{e[1]})
}

func TestIssue4102(t *testing.T) {
	e1 := SliceOfElements(1, 2, 8)
	l1 := New()
	l1.PushBack(e1[0])
	l1.PushBack(e1[1])

	e2 := SliceOfElements(3, 4)
	l2 := New()
	l2.PushBack(e2[0])
	l2.PushBack(e2[1])

	ef1 := l1.Front()
	l2.Remove(ef1) // l2 should not change because ef1 is not an element of l2
	if n := l2.Len(); n != 2 {
		t.Errorf("l2.Len() = %d, want 2", n)
	}
	l1.InsertBefore(e1[2], ef1)
	if n := l1.Len(); n != 3 {
		t.Errorf("l1.Len() = %d, want 3", n)
	}
}

func TestIssue6349(t *testing.T) {
	l := New()
	l.PushBack(NewElement(1))
	l.PushBack(NewElement(2))
	e := l.Front()
	l.Remove(e)
	el := e.(*Element)
	i := el.Value.(int)
	if i != 1 {
		t.Errorf("e.value = %d, want 1", i)
	}
	if e.Next() != nil {
		t.Errorf("e.Next() != nil")
	}
	if e.Prev() != nil {
		t.Errorf("e.Prev() != nil")
	}
	if e.List() != nil {
		t.Errorf("e.List() != nil")
	}

}

func TestMove(t *testing.T) {
	l := New()
	e1 := NewElement(1)
	e2 := NewElement(2)
	e3 := NewElement(3)
	e4 := NewElement(4)

	l.PushBack(e1)
	l.PushBack(e2)
	l.PushBack(e3)
	l.PushBack(e4)

	l.MoveAfter(e3, e3) // should be no-op
	checkListPointers(t, "Check 4 element list", l, []*Element{e1, e2, e3, e4})
	l.MoveBefore(e2, e2) // shold be a no-op
	checkListPointers(t, "Check 4 element list no-op", l, []*Element{e1, e2, e3, e4})

	l.MoveAfter(e3, e2)
	checkListPointers(t, "Check 4 element list after e3,e2", l, []*Element{e1, e2, e3, e4})
	l.MoveBefore(e2, e3)
	checkListPointers(t, fmt.Sprintf("Check 4 element list before e2(%p),e3(%p)", e2, e3), l, []*Element{e1, e2, e3, e4})

	l.MoveBefore(e2, e4)
	checkListPointers(t, "Check 4 element list before e2,e4", l, []*Element{e1, e3, e2, e4})
	e1, e2, e3, e4 = e1, e3, e2, e4

	l.MoveBefore(e4, e1)
	checkListPointers(t, "Check 4 element list before e4,e1", l, []*Element{e4, e1, e2, e3})
	e1, e2, e3, e4 = e4, e1, e2, e3

	l.MoveAfter(e4, e1)
	checkListPointers(t, "Check 4 element list after e4,e1", l, []*Element{e1, e4, e2, e3})
	e1, e2, e3, e4 = e1, e4, e2, e3

	l.MoveAfter(e2, e3)
	checkListPointers(t, "Check 4 element list after e2,e3", l, []*Element{e1, e3, e2, e4})
	e1, e2, e3, e4 = e1, e3, e2, e4

}

// Tst PushFront, PushBack with nil values and uninitialized list..
func TestZeroList(t *testing.T) {
	var l1 = new(List)
	l1.PushFront(NewElement(1))
	checkList(t, "PushFront uninit list", l1, []int{1})

	l1.PushFront(nil)
	checkList(t, "PushFront nil value", l1, []int{1})

	var l2 = new(List)
	l2.PushBack(NewElement(1))
	checkList(t, "PushBack uninit list", l2, []int{1})

	l2.PushBack(nil)
	checkList(t, "PushBack nil", l2, []int{1})

}

// Test that a list l is not modified when calling InsertBefore with a mark that is not an element of the list.
func TestInsertBeforeUnknownMark(t *testing.T) {
	var l List
	l.PushBack(NewElement(1))
	l.PushBack(NewElement(2))
	l.PushBack(NewElement(3))
	l.InsertBefore(NewElement(4), new(Element))
	checkList(t, "Check insert before unknown mark", &l, []int{1, 2, 3})
}

// Test that a list l is not modified when calling InsertAfter with a mark that is not an element of the list.
func TestInsertAfterUnknownMark(t *testing.T) {
	var l List
	l.PushBack(NewElement(1))
	l.PushBack(NewElement(2))
	l.PushBack(NewElement(3))
	l.InsertAfter(NewElement(4), new(Element))
	checkList(t, "Check insert after unknown mark", &l, []int{1, 2, 3})
}

// Test that a list l is not modified when calling InsertBefore or InsertAfter with a mark that is not an element of the list.
func TestMoveUnknownMark(t *testing.T) {
	var l1, l2 List

	e1, e2 := NewElement(1), NewElement(2)
	l1.PushBack(e1)
	l2.PushBack(e2)

	l1.MoveAfter(e1, e2)
	checkList(t, "Check move after unknown mark l1", &l1, []int{1})
	checkList(t, "Check move after unknown mark l2", &l2, []int{2})

	l1.MoveBefore(e1, e2)
	checkList(t, "Check move before unknown mark l1", &l1, []int{1})
	checkList(t, "Check move before unknown mark l2", &l2, []int{2})
}
