package list

// Sentinel Provides the a very basic struct that fulfills the Elementer interface. It is meant to be embedded into other
// structs that want to fulfill the interface.
type Sentinel struct {
	next Elementer
	list *List
}

// Returns the next Element in the list.
func (s *Sentinel) Next() Elementer {
	if s == nil {
		return nil
	}
	return s.next
}

// Sets the next element in the list.
func (s *Sentinel) SetNext(e Elementer) Elementer {
	if s == nil {
		return nil
	}
	n := s.next
	s.next = e
	return n
}

// Returns the list.
func (s *Sentinel) List() *List {
	if s == nil {
		return nil
	}
	return s.list
}

// Set's the list.
func (s *Sentinel) SetList(l *List) *List {
	if s == nil {
		return nil
	}
	ol := s.list
	s.list = l
	return ol
}
