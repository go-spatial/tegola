//go:build ignore
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

var (
	showOutput   bool
	bindataDebug bool
)

func init() {
	flag.BoolVar(&showOutput, "show-output", false, "show command output")
	flag.BoolVar(&bindataDebug, "bindata-debug", false, "generate bin-data in debug mode")
}

func printOutput(out []byte) {
	lines := strings.Split(string(out), "\n")
	for i := range lines {
		fmt.Printf("> %s\n", lines[i])
	}
}

func runCmd(cmdName string, parts ...string) ([]byte, error) {
	cmdLine := strings.Join(append([]string{cmdName}, parts...), " ")
	fmt.Printf("Running: %v\n", cmdLine)
	cmd := exec.Command(cmdName, parts...)
	return cmd.CombinedOutput()
}

func run(cmdName string, parts ...string) {
	cmdLine := strings.Join(append([]string{cmdName}, parts...), " ")
	out, err := runCmd(cmdName, parts...)
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
	// Check to see if npm is installed.
	if _, err := runCmd("npm", "version"); err != nil {
		fmt.Printf("Did not find npm, not running...")
		os.Exit(0)
	}

	// install npm dependencies
	run("npm", "install")
	// build app
	run("npm", "run", "build")

	// make sure .keep file remains in the dist directory
	f, err := os.OpenFile("dist/.keep", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		fmt.Printf("failed to create dist/.keep file")
	} else {
		f.Close()
	}

	fmt.Printf("success\n")
}
