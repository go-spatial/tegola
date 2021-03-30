package build

import (
	"sort"
	"strings"
)

var (
	Version     = "Version not set"
	GitRevision = "not set"
	GitBranch   = "not set"
	Tags        []string
	Commands    = []string{"tegola"}
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
