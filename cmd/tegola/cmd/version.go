package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of tegola",
	Long:  `All software has versions, so in order for tegola to be considered software...`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
