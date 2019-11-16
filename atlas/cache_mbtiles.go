// +build !noMBTilesCache

package atlas

// The point of this file is to load and register the mbtiles cache backend.
// the mbtiles cache can be excluded during the build with the `noMBTilesCache` build flag
// for example from the cmd/tegola directory:
//
// go build -tags 'noMBTilesCache'
import (
	_ "github.com/go-spatial/tegola/cache/mbtiles"
)
