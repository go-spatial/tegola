//go:build tools

package tools

// goveralls is a CI-only tool, installed via `go install ...@v0.0.5` in the
// GitHub Actions workflow (see .github/workflows/on_pr_push.yml). It is
// intentionally NOT listed in go.mod require to keep the main module graph
// clean; this file previously imported it to pin it as a tools dependency.
