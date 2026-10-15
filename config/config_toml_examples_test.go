package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
)

// TestParseRepoTomlConfigs guards against TOML library upgrades (BurntSushi
// v0.4 -> v1 changed decoding strictness) by parsing every TOML config
// shipped in the repository: config/testdata fixtures, the root example
// config, and the provider testdata configs (R10).
func TestParseRepoTomlConfigs(t *testing.T) {
	// test_env.toml references ENV_TEST_* variables; mirror TestParse setup
	setEnv(t)

	repoRoot, err := filepath.Abs(filepath.Join(".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	tomls, err := filepath.Glob(filepath.Join(repoRoot, "config", "testdata", "*.toml"))
	if err != nil {
		t.Fatalf("glob config/testdata: %v", err)
	}
	for _, pattern := range []string{
		filepath.Join(repoRoot, "tegola.config.toml"),
		filepath.Join(repoRoot, "testdata", "*", "*.toml"),
		filepath.Join(repoRoot, "provider", "*", "testdata", "*.toml"),
	} {
		matches, gerr := filepath.Glob(pattern)
		if gerr != nil {
			t.Fatalf("glob %v: %v", pattern, gerr)
		}
		tomls = append(tomls, matches...)
	}
	if len(tomls) == 0 {
		t.Fatal("no toml configs found at repo root: test walked the wrong directory?")
	}

	type tcase struct {
		path       string
		expectErr  bool
		errMessage string
	}

	var tests []tcase
	for _, p := range tomls {
		tc := tcase{path: p}
		// missing_env.toml references an env var that is intentionally not
		// set; parsing must fail with that variable's name
		if strings.HasSuffix(p, "missing_env.toml") {
			tc.expectErr = true
			tc.errMessage = "I_AM_MISSING"
		}
		tests = append(tests, tc)
	}

	for _, tc := range tests {
		path := tc.path
		expectErr := tc.expectErr
		errMessage := tc.errMessage
		t.Run(filepath.Join(relRepo(repoRoot, path)), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() { _ = f.Close() }()

			_, err = config.Parse(f, path)
			if expectErr {
				if err == nil {
					t.Fatal("expected parse error, got nil")
				}
				if errMessage != "" && !strings.Contains(err.Error(), errMessage) {
					t.Errorf("parse error %q does not mention %q", err, errMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
		})
	}
}

// relRepo renders path relative to the repo root for readable subtest names.
func relRepo(repoRoot, path string) string {
	if rel, err := filepath.Rel(repoRoot, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}
