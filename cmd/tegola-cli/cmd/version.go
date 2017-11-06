package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of tegola",
	Long:  `All software has versions. Meet tegola`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
