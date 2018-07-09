// +build !noAzblobCache

package atlas

// The point of this file is to load and register the azblob cache backend.
// the azblob cache can be excluded during the build with the `noAzblobCache` build flag
// for example from the cmd/tegola direcotry:
//
// go build -tags 'noAzblobCache'
import (
	_ "github.com/go-spatial/tegola/cache/azblob"
)
