// +build !noRedisCache

package atlas

// The point of this file is to load and register the redis cache backend.
// the redis cache can be excluded during the build with the `noRedisCache` build flag
// for example from the cmd/tegola directory:
//
// go build -tags 'noRedisCache'
import (
	_ "github.com/go-spatial/tegola/cache/redis"
)
