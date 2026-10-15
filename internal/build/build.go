package build

//go:generate go run tags/tags.go -v -runCommand="internal/build/tags.go" -source=../..

import (
	"sort"
	"strings"
)

var (
	// Version is the build version. It is normally injected at build time via
	// ldflags (-X github.com/alexeydott/tegola/internal/build.Version=...). The
	// value below is only the fallback used when the binary is built without that
	// injection (e.g. a plain `go build`). Keep it in sync with server.Version.
	// Fork scheme is v0.21.0-fork.N (upstream base = upstream master post-v0.21.0);
	// see UPSTREAM.md for provenance.
	Version              = "v0.21.0-fork.1"
	GitRevision          = "not set"
	GitBranch            = "not set"
	uiVersionDefaultText = "viewer not built"
	Tags                 []string
	Commands             = []string{"tegola"}
)

var ordered bool

func OrderedTags() []string {
	if !ordered {
		sort.Strings(Tags)
		ordered = true
	}
	return Tags
}

func Command() string { return strings.Join(Commands, " ") }
