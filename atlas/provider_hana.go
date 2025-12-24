//go:build !noHanaProvider
// +build !noHanaProvider

package atlas

// The point of this file is to load and register the HANA provider.
// the HANA provider can be excluded during the build with the `noHanaProvider` build flag
// for example from the cmd/tegola directory:
//
// go build -tags 'noHanaProvider'
import (
	_ "github.com/go-spatial/tegola/provider/hana"
)
