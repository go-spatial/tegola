//go:build !cgo

package gpkg

import (
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func NewTileProvider(config dict.Dicter, maps []provider.Map) (provider.Tiler, error) {
	return nil, provider.ErrUnsupported
}

func Cleanup() {}
