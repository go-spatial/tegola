// +build !cgo

package mbtiles

import (
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
)

func New(config dict.Dicter) (cache.Interface, error) {
	return nil, cache.ErrUnsupported
}
