package atlas

// The point of this file is to load and register the default cache backends
import (
	_ "github.com/alexeydott/tegola/cache/file"
	_ "github.com/alexeydott/tegola/cache/memory"
	_ "github.com/alexeydott/tegola/cache/multilevel"
)
