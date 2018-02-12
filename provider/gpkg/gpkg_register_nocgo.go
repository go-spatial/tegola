// +build !cgo

package gpkg

import "github.com/terranodo/tegola/provider"

func NewTileProvider(config map[string]interface{}) (provider.Tiler, error) {
	return nil, provider.ErrUnsupported
}
