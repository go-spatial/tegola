package gpkg

import (
	"errors"
	"fmt"
)

var (
	ErrMissingLayerName = errors.New("gpkg: layer is missing 'name'")
)

type ErrInvalidFilePath struct {
	FilePath string
}

func (e ErrInvalidFilePath) Error() string {
	return fmt.Sprintf("gpkg: invalid filepath: %v", e.FilePath)
}
