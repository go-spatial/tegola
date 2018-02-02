package main

import (
	"fmt"
	"os"

	"github.com/terranodo/tegola/cmd/xyz2svg/cmd"
)

func main() {
	if err := cmd.Root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
