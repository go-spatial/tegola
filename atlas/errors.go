package atlas

import (
	"errors"
	"fmt"
)

type ErrMapNotFound struct {
	Name string
}

func (e ErrMapNotFound) Error() string {
	return fmt.Sprintf("atlas: map (%v) not found", e.Name)
}

var ErrMissingCache = errors.New("atlas: missing cache")
