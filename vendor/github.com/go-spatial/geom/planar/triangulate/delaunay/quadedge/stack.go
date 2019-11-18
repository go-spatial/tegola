package quadedge

// Stack is a stack of edges
type Stack []*Edge

// Push will add an edge to the stack
func (s *Stack) Push(e *Edge) {
	if s == nil {
		return
	}
	*s = append(*s, e)
}

// Pop will return the last edge added and remove it from the stack.
func (s *Stack) Pop() (e *Edge) {
	if s == nil || len(*s) == 0 {
		return nil
	}
	e = (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return e
}

// Length will return the length of the stack
func (s Stack) Length() int { return len(s) }
