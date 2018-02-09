package atlas

import (
	"errors"
	"fmt"
)

var (
	ErrMissingCache = errors.New("atlas: missing cache")
	ErrMissingTile  = errors.New("atlas: missing tile")
)

type ErrMapNotFound struct {
	Name string
}

func (e ErrMapNotFound) Error() string {
	return fmt.Sprintf("atlas: map (%v) not found", e.Name)
}
