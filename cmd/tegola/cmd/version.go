package cmd

import (
	"fmt"
	"github.com/go-spatial/cobra"
	"github.com/go-spatial/tegola/internal/build"
	"strings"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of tegola",
	Long:  `All software has versions, so in order for tegola to be considered software...`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("   version: %s\n", build.Version)
		fmt.Printf("       git: %s @ %v\n", build.GitBranch, build.GitRevision)
		fmt.Printf("build tags: %s\n", strings.Join(build.OrderedTags(), " "))
		fmt.Printf(" ui viewer: %s\n", build.ViewerVersion())
	},
}
