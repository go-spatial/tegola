// +build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

var showOutput bool

func init() {
	flag.BoolVar(&showOutput, "show-output", false, "show command output")
}

func printOutput(out []byte) {
	lines := strings.Split(string(out), "\n")
	for i := range lines {
		fmt.Printf("> %s\n", lines[i])
	}
}

func run(cmdName string, parts ...string) {

	cmdLine := strings.Join(append([]string{cmdName}, parts...), " ")
	fmt.Printf("Running: %v\n", cmdLine)
	cmd := exec.Command(cmdName, parts...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		printOutput(out)
		fmt.Printf("PATH: %s\n", os.Getenv(`PATH`))
		fmt.Printf("While running `%s`\nGot error:\n%v\n", cmdLine, err)
		os.Exit(2)
	}
	if showOutput {
		printOutput(out)
	}
}

// fileDir will return the path the where this file is located
func fileDir() string {

	_, filename, _, _ := runtime.Caller(0)
	return path.Dir(filename)
}

func main() {
	flag.Parse()
	// Change into the directory this file lives in.
	if err := os.Chdir(fileDir()); err != nil {
		fmt.Printf("Failed to change dir to: %v\nerror: %v\n", fileDir(), err)
		os.Exit(3)
	}
	fmt.Printf("Changed to directory: %v\n", fileDir())

	// install npm dependences
	run("npm", "install")
	// build app
	run("npm", "run", "build")
	// build bindata
	run("go-bindata", "-pkg=bindata", "-o=../server/bindata/bindata.go", "-ignore=.DS_Store", "dist/...")
	fmt.Printf("success\n")
}
