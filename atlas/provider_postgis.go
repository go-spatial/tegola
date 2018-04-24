// +build !noPostgisProvider

package atlas

// The point of this file is to load and register the PostGIS provider.
// the PostGIS provider can be excluded during the build with the `noPostgisProvider` build flag
// for example from the cmd/tegola direcotry:
//
// go build -tags 'noPostgisProvider'
import (
	_ "github.com/go-spatial/tegola/provider/postgis"
)
