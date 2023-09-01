package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/internal/log"
)

// FileConfigSource is a config source for loading and watching files in a local directory.
type FileConfigSource struct {
	dir string
}

func (s *FileConfigSource) Type() string {
	return "file"
}

func (s *FileConfigSource) Init(options env.Dict, baseDir string) error {
	var err error
	dir, err := options.String("dir", nil)
	if err != nil {
		return err
	}

	// If dir is relative, make it relative to baseDir.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(baseDir, dir)
	}

	s.dir = dir
	return nil
}

// LoadAndWatch will read all the files in the configured directory and then keep watching the directory for changes.
func (s *FileConfigSource) LoadAndWatch(ctx context.Context) (ConfigWatcher, error) {
	appWatcher := ConfigWatcher{
		Updates:   make(chan App),
		Deletions: make(chan string),
	}

	// First check that the directory exists and is readable.
	if _, err := os.ReadDir(s.dir); err != nil {
		return appWatcher, fmt.Errorf("Apps directory not readable: %s", err)
	}

	// Now setup the filesystem watcher.
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return appWatcher, err
	}

	err = fsWatcher.Add(s.dir)
	if err != nil {
		return appWatcher, err
	}

	go func() {
		defer fsWatcher.Close()

		// First load the files already present in the directory.
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			log.Errorf("Could not read apps directory (%s). Exiting watcher. %s", s.dir, err)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
				log.Debug("Ignoring ", entry.Name())
				continue
			}

			log.Infof("Loading app file %s...", entry.Name())
			s.loadApp(filepath.Join(s.dir, entry.Name()), appWatcher.Updates)
		}

		// Now start processing future additions/removals/edits.
		for {
			select {
			case event, ok := <-fsWatcher.Events:
				if !ok {
					return
				}

				if !strings.HasSuffix(event.Name, ".toml") {
					log.Debug("Ignoring ", event.Name)
					continue
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Infof("Loading app file %s (%s)...", event.Name, event.Op)
					s.loadApp(event.Name, appWatcher.Updates)
				} else if event.Has(fsnotify.Remove) {
					log.Infof("Unloading app file %s (%s)...", event.Name, event.Op)
					appWatcher.Deletions <- event.Name
				}

			case err, ok := <-fsWatcher.Errors:
				if !ok {
					return
				}
				log.Error(err)

			case <-ctx.Done():
				log.Info("Exiting watcher...")
				return
			}
		}
	}()

	return appWatcher, nil
}

// loadApp reads the file and loads the app into the updates channel.
func (s *FileConfigSource) loadApp(filename string, updates chan App) {
	f, err := os.Open(filename)
	if err != nil {
		log.Errorf("Failed to load %s: %s", filename, err)
		return
	}
	defer f.Close()

	if app, err := parseApp(f, filename); err == nil {
		updates <- app
	} else {
		log.Errorf("Failed to parse %s: %s", filename, err)
	}
}
