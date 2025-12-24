//go:build !cgo
// +build !cgo

package gpkg

import "github.com/go-spatial/tegola/provider"

func NewTileProvider(config map[string]interface{}) (provider.Tiler, error) {
	return nil, provider.ErrUnsupported
}

func Cleanup() {}
