package atlas

// The point of this file is to load and register the cache backend we support.
import (
	_ "github.com/go-spatial/tegola/cache/file"
	_ "github.com/go-spatial/tegola/cache/redis"
	_ "github.com/go-spatial/tegola/cache/s3"
)
