//go:build !cgo
// +build !cgo

package gpkg

import (
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/provider"
)

func NewTileProvider(config dict.Dicter, maps []provider.Map) (provider.Tiler, error) {
	return nil, provider.ErrUnsupported
}

func Cleanup() {}
