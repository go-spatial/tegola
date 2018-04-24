// +build !noGpkgProvider

package atlas

// The point of this file is to load and register the GeoPackage provider.
// the GeoPackage provider can be excluded during the build with the `noGpkgProvider` build flag
// for example from the cmd/tegola direcotry:
//
// go build -tags 'noGpkgProvider'
import (
	_ "github.com/go-spatial/tegola/provider/gpkg"
)
