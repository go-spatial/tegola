package quadedge

// ErrInvalid is returned when the type is invalid and the reason
// why it's invalid
type ErrInvalid []string

// Error fullfils the errorer interface
func (err ErrInvalid) Error() string { return "invalid" }
