package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	versionFile    = flag.String("version_file", "", "The file that contains the version number")
	revisionString = flag.String("revision", gitRevision(), "the revision name")
	branchString   = flag.String("branch", gitBranch(), "the branch name")
	versionPkg     = flag.String("pkg", "github.com/go-spatial/tegola/internal/build", "the package where the version is set")
)

func runCmd(cmdName string, parts ...string) ([]byte, error) {
	cmd := exec.Command(cmdName, parts...)
	return cmd.CombinedOutput()
}

func gitRevision() string {
	gitRevision, err := runCmd("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(gitRevision))
}

func gitBranch() string {
	gitBranch, err := runCmd("git", "branch", "--no-color", "--show-current")
	if err != nil {
		panic(err)
	}
	b := strings.TrimSpace(string(gitBranch))
	if b == "" {
		return "detached"
	}
	return b
}
func version() string {
	if *versionFile == "" {
		return ""
	}
	data, err := os.ReadFile(*versionFile)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(data))
}

func main() {
	flag.Parse()
	ver := version()
	if ver == "" {
		ver = *revisionString
	}
	fmt.Printf(`VERSION=%s
GIT_BRANCH=%s
GIT_REVISION=%s
BUILD_PKG=%s
`,
		ver,
		*branchString,
		*revisionString,
		*versionPkg,
	)
}
