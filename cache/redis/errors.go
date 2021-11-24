package redis

// ErrHostMissing is raised when Redis Addr is missing Host
type ErrHostMissing struct {
	msg string
}

func (error *ErrHostMissing) Error() string {
	return error.msg
}
