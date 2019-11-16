// +build !cgo

package file

import (
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
)

func New(config dict.Dicter) (cache.Interface, error) {
	return nil, cache.ErrUnsupported
}
