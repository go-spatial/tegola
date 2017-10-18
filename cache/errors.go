package cache

import "fmt"

type ErrInvalidFileKey struct {
	path          string
	keyPartsCount int
}

func (e ErrInvalidFileKey) Error() string {
	return fmt.Sprintf("cache: invalid fileKey (%v). expecting between three and five parts, got (%v) skipping.", e.path, e.keyPartsCount)
}
