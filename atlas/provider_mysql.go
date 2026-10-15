//go:build !noMysqlProvider
// +build !noMysqlProvider

package atlas

// The point of this file is to load and register the MySQL provider.
// the MySQL provider can be excluded during the build with the `noMysqlProvider` build flag
// for example from the cmd/tegola directory:
//
// go build -tags 'noMysqlProvider'
import (
	_ "github.com/alexeydott/tegola/provider/mysql"
)
