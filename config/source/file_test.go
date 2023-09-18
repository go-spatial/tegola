package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-spatial/tegola/internal/env"
)

func TestFileConfigSourceInit(t *testing.T) {
	var (
		src FileConfigSource
		err error
	)

	src = FileConfigSource{}
	err = src.init(env.Dict{}, "")
	if err == nil {
		t.Error("init() should return an error if no dir provided; no error returned.")
	}

	absDir := "/tmp/configs"
	src = FileConfigSource{}
	src.init(env.Dict{"dir": absDir}, "/opt")
	if src.dir != absDir {
		t.Errorf("init() should preserve absolute path %s; found %s instead.", absDir, src.dir)
	}

	relDir := "configs"
	src = FileConfigSource{}
	src.init(env.Dict{"dir": relDir}, "/root")
	joined := filepath.Join("/root", relDir)
	if src.dir != joined {
		t.Errorf("init() should place relative path under given basedir; expected %s, found %s.", joined, src.dir)
	}
}

func TestFileConfigSourceLoadAndWatch(t *testing.T) {
	var (
		src     FileConfigSource
		watcher ConfigWatcher
		dir     string
		ctx     context.Context
		err     error
	)

	dir, _ = os.MkdirTemp("", "tegolaapps")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should error if directory not readable.
	src = FileConfigSource{}
	src.init(env.Dict{"dir": filepath.Join(dir, "nonexistent")}, "")
	watcher, err = src.LoadAndWatch(ctx)
	if err == nil {
		t.Error("LoadAndWatch() should error if directory doesn't exist; no error returned.")
	}

	// Should load files already present in directory.
	err = createFile(App{}, filepath.Join(dir, "app1.toml"))
	if err != nil {
		t.Errorf("Could not create an app config file. %s", err)
		return
	}
	src = FileConfigSource{}
	src.init(env.Dict{"dir": dir}, "")
	watcher, err = src.LoadAndWatch(ctx)
	if err != nil {
		t.Errorf("No error expected from LoadAndWatch(): returned %s", err)
		return
	}

	updates := readAllUpdates(watcher.Updates)
	if len(updates) != 1 || updates[0].Key != filepath.Join(dir, "app1.toml") {
		t.Errorf("Failed reading preexisting files: len=%d updates=%v", len(updates), updates)
	}

	// Should detect new files added to directory.
	createFile(App{}, filepath.Join(dir, "app2.toml"))
	createFile(App{}, filepath.Join(dir, "app3.toml"))
	updates = readAllUpdates(watcher.Updates)
	if len(updates) != 2 || updates[0].Key != filepath.Join(dir, "app2.toml") || updates[1].Key != filepath.Join(dir, "app3.toml") {
		t.Errorf("Failed reading new files: len=%d updates=%v", len(updates), updates)
	}

	// Should detect files removed from directory.
	os.Remove(filepath.Join(dir, "app2.toml"))
	os.Remove(filepath.Join(dir, "app1.toml"))
	deletions := readAllDeletions(watcher.Deletions)
	if len(deletions) != 2 || !contains(deletions, filepath.Join(dir, "app2.toml")) || !contains(deletions, filepath.Join(dir, "app1.toml")) {
		t.Errorf("Failed detecting deletions: len=%d deletions=%v", len(deletions), deletions)
	}
}

func TestFileConfigSourceLoadAndWatchShouldExitOnDone(t *testing.T) {
	dir, _ := os.MkdirTemp("", "tegolaapps")

	src := FileConfigSource{}
	src.init(env.Dict{"dir": dir}, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := src.LoadAndWatch(ctx)
	if err != nil {
		t.Errorf("No error expected from LoadAndWatch(): returned %s", err)
		return
	}

	cancel()
	select {
	case <-watcher.Updates:
		// do nothing
	case <-time.After(time.Millisecond):
		t.Error("Updates channel should be closed, but is still open.")
	}

	select {
	case <-watcher.Deletions:
		// do nothing
	case <-time.After(time.Millisecond):
		t.Error("Deletions channel should be closed, but is still open.")
	}
}

func createFile(app App, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	err = toml.NewEncoder(f).Encode(app)
	return err
}

func readAllUpdates(updates chan App) []App {
	apps := []App{}

	for {
		select {
		case app, ok := <-updates:
			if !ok {
				return apps
			}

			apps = append(apps, app)

		case <-time.After(10 * time.Millisecond):
			return apps
		}
	}
}

func readAllDeletions(deletions chan string) []string {
	keys := []string{}

	for {
		select {
		case key, ok := <-deletions:
			if !ok {
				return keys
			}

			keys = append(keys, key)

		case <-time.After(10 * time.Millisecond):
			return keys
		}
	}
}

func contains(vals []string, expected string) bool {
	for _, str := range vals {
		if str == expected {
			return true
		}
	}

	return false
}
