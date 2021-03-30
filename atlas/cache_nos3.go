// +build noS3Cache

package atlas

import "github.com/go-spatial/tegola/internal/build"

func init() {
	// add ourself to the build
	build.Tags = append(build.Tags, "noS3Cache")
}
