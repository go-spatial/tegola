package main

import (
	"fmt"
	"os"

	_ "github.com/theckman/goconstraint/go1.8/gte"

	"github.com/go-spatial/tegola/cmd/tegola/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
