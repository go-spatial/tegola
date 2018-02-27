package main

import (
	"fmt"
	"os"

	"github.com/go-spatial/tegola/cmd/tegola/cmd"
)

var codeLogFile *os.File

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
