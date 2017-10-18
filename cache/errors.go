package cache

import "fmt"

type ErrInvalidFileKeyParts struct {
	path          string
	keyPartsCount int
}

func (e ErrInvalidFileKeyParts) Error() string {
	return fmt.Sprintf("cache: invalid fileKey (%v). expecting between three and five parts, got (%v) skipping.", e.path, e.keyPartsCount)
}

type ErrInvalidFileKey struct {
	path string
	key  string
	val  string
}

func (e ErrInvalidFileKey) Error() string {
	return fmt.Sprintf("cache: invalid fileKey (%v). unable to parse (%v) value (%v) into int", e.path, e.key, e.val)
}
