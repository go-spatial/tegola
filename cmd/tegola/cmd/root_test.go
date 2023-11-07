package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/config/source"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider"
)

type initializerMock struct {
	initProvidersCalls chan bool
	initMapsCalls      chan bool
	unloadCalls        chan bool
}

func (i initializerMock) initProviders(providersConfig []env.Dict, maps []provider.Map, namespace string) (map[string]provider.TilerUnion, error) {
	i.initProvidersCalls <- true
	return map[string]provider.TilerUnion{}, nil
}

func (i initializerMock) initProvidersCalled() bool {
	select {
	case _, ok := <-i.initProvidersCalls:
		return ok
	case <-time.After(time.Millisecond):
		return false
	}
}

func (i initializerMock) initMaps(maps []provider.Map, providers map[string]provider.TilerUnion) error {
	i.initMapsCalls <- true
	return nil
}

func (i initializerMock) initMapsCalled() bool {
	select {
	case _, ok := <-i.initMapsCalls:
		return ok
	case <-time.After(time.Millisecond):
		return false
	}
}

func (i initializerMock) unload(app source.App) {
	i.unloadCalls <- true
}

func (i initializerMock) unloadCalled() bool {
	select {
	case _, ok := <-i.unloadCalls:
		return ok
	case <-time.After(time.Millisecond):
		return false
	}
}

func TestInitAppConfigSource(t *testing.T) {
	var (
		cfg config.Config
		err error
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should return nil if app config source type not specified.
	cfg = config.Config{}
	err = initAppConfigSource(ctx, cfg)
	if err != nil {
		t.Errorf("Unexpected error when app config source type is not specified: %s", err)
	}

	// Should return error if unable to initialize source.
	cfg = config.Config{
		AppConfigSource: env.Dict{
			"type": "something_nonexistent",
		},
	}
	err = initAppConfigSource(ctx, cfg)
	if err == nil {
		t.Error("Should return an error if invalid source type provided")
	}

	cfg = config.Config{
		AppConfigSource: env.Dict{
			"type": "file",
			"dir":  "something_nonexistent",
		},
	}
	err = initAppConfigSource(ctx, cfg)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Errorf("Should return an error if unable to initialize source; expected an error about the directory, got %v", err)
	}
}

func TestWatchAppUpdates(t *testing.T) {
	loader := initializerMock{
		initProvidersCalls: make(chan bool),
		initMapsCalls:      make(chan bool),
		unloadCalls:        make(chan bool),
	}
	// watcher := mock.NewWatcherMock()
	watcher := source.ConfigWatcher{
		Updates:   make(chan source.App),
		Deletions: make(chan string),
	}
	defer watcher.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watchAppUpdates(ctx, watcher, loader)

	// Should load new map+provider
	app := source.App{
		Providers: []env.Dict{},
		Maps:      []provider.Map{},
		Key:       "Test1",
	}
	// watcher.SendUpdate(app)
	watcher.Updates <- app
	if !loader.initProvidersCalled() {
		t.Error("Failed to initialize providers")
	}
	if !loader.initMapsCalled() {
		t.Error("Failed to initialize maps")
	}
	if loader.unloadCalled() {
		t.Error("Unexpected app unload")
	}

	// Should load updated map+provider
	// watcher.SendUpdate(app)
	watcher.Updates <- app
	if !loader.unloadCalled() {
		t.Error("Failed to unload old app")
	}
	if !loader.initProvidersCalled() {
		t.Error("Failed to initialize providers")
	}
	if !loader.initMapsCalled() {
		t.Error("Failed to initialize maps")
	}

	// Should unload map+provider
	// watcher.SendDeletion("Test1")
	watcher.Deletions <- "Test1"
	if !loader.unloadCalled() {
		t.Error("Failed to unload old app")
	}
}

func TestGetMapNames(t *testing.T) {
	app := source.App{
		Maps: []provider.Map{
			{Name: "First Map"},
			{Name: "Second Map"},
		},
	}
	expected := []string{"First Map", "Second Map"}
	names := getMapNames(app)
	if !reflect.DeepEqual(expected, names) {
		t.Errorf("Expected map names %v; found %v", expected, names)
	}
}

func TestGetProviderNames(t *testing.T) {
	var (
		app      source.App
		names    []string
		expected []string
	)

	// Happy path
	app = source.App{
		Providers: []env.Dict{
			{"name": "First Provider"},
			{"name": "Second Provider"},
		},
	}
	expected = []string{"First Provider", "Second Provider"}
	names = getProviderNames(app)
	if !reflect.DeepEqual(expected, names) {
		t.Errorf("Expected provider names %v; found %v", expected, names)
	}

	// Error
	app = source.App{
		Providers: []env.Dict{
			{},
			{"name": "Second Provider"},
		},
	}
	expected = []string{"Second Provider"}
	names = getProviderNames(app)
	if !reflect.DeepEqual(expected, names) {
		t.Errorf("Expected provider names %v; found %v", expected, names)
	}
}
