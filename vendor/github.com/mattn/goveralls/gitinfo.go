package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
)

// A Head object encapsulates information about the HEAD revision of a git repo.
type Head struct {
	ID             string `json:"id"`
	AuthorName     string `json:"author_name,omitempty"`
	AuthorEmail    string `json:"author_email,omitempty"`
	CommitterName  string `json:"committer_name,omitempty"`
	CommitterEmail string `json:"committer_email,omitempty"`
	Message        string `json:"message"`
}

// A Git object encapsulates information about a git repo.
type Git struct {
	Head   Head   `json:"head"`
	Branch string `json:"branch"`
}

// collectGitInfo runs several git commands to compose a Git object.
func collectGitInfo(ref string) *Git {
	gitCmds := map[string][]string{
		"id":      {"rev-parse", ref},
		"branch":  {"branch", "--format", "%(refname:short)", "--contains", ref},
		"aname":   {"show", "-s", "--format=%aN", ref},
		"aemail":  {"show", "-s", "--format=%aE", ref},
		"cname":   {"show", "-s", "--format=%cN", ref},
		"cemail":  {"show", "-s", "--format=%cE", ref},
		"message": {"show", "-s", "--format=%s", ref},
	}
	results := map[string]string{}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		log.Printf("fail to look path of git: %v", err)
		log.Print("git information is omitted")
		return nil
	}

	if ref != "HEAD" {
		// make sure that the commit is in the local
		// e.g. shallow cloned repository
		_, _ = runCommand(gitPath, "fetch", "--depth=1", "origin", ref)
		// ignore errors because we don't have enough information about the origin.
	}

	for key, args := range gitCmds {
		if key == "branch" {
			if envBranch := loadBranchFromEnv(); envBranch != "" {
				results[key] = envBranch
				continue
			}
		}

		ret, err := runCommand(gitPath, args...)
		if err != nil {
			log.Printf(`fail to run "%s %s": %v`, gitPath, strings.Join(args, " "), err)
			log.Print("git information is omitted")
			return nil
		}
		results[key] = ret
	}
	h := Head{
		ID:             firstLine(results["id"]),
		AuthorName:     firstLine(results["aname"]),
		AuthorEmail:    firstLine(results["aemail"]),
		CommitterName:  firstLine(results["cname"]),
		CommitterEmail: firstLine(results["cemail"]),
		Message:        results["message"],
	}
	g := &Git{
		Head:   h,
		Branch: firstLine(results["branch"]),
	}
	return g
}

func runCommand(gitPath string, args ...string) (string, error) {
	cmd := exec.Command(gitPath, args...)
	ret, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	ret = bytes.TrimRight(ret, "\n")
	return string(ret), nil
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

var varNames = [...]string{
	"GIT_BRANCH",

	// https://help.github.com/en/actions/automating-your-workflow-with-github-actions/using-environment-variables
	"GITHUB_HEAD_REF", "GITHUB_REF",

	"CIRCLE_BRANCH", "TRAVIS_BRANCH",
	"CI_BRANCH", "APPVEYOR_REPO_BRANCH",
	"WERCKER_GIT_BRANCH", "DRONE_BRANCH",
	"BUILDKITE_BRANCH", "BRANCH_NAME",
}

func loadBranchFromEnv() string {
	for _, varName := range varNames {
		if branch := os.Getenv(varName); branch != "" {
			if varName == "GITHUB_REF" {
				return strings.TrimPrefix(branch, "refs/heads/")
			}
			return branch
		}
	}

	return ""
}
