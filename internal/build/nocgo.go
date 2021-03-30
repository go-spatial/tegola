// +build !cgo

package build

func init() {
	// add ourself to the build
	build.Tags = append(build.Tags, "noCGO")
}
