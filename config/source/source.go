package source

import (
	"context"
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider"
)

// App represents a set of providers and maps that should be added/removed together.
type App struct {
	Providers []env.Dict     `toml:"providers"`
	Maps      []provider.Map `toml:"maps"`
	Key       string         // key is used to track this app through its lifecycle and could be anything to uniquely identify it.
}

type ConfigSource interface {
	Type() string
	LoadAndWatch(ctx context.Context) (ConfigWatcher, error)
}

type ConfigWatcher struct {
	Updates   chan App
	Deletions chan string
}

func InitSource(sourceType string, options env.Dict, baseDir string) (ConfigSource, error) {
	switch sourceType {
	case "file":
		src := FileConfigSource{}
		err := src.Init(options, baseDir)
		return &src, err

	default:
		return nil, fmt.Errorf("No ConfigSource of type %s", sourceType)
	}
}

// parseApp decodes any reader into an App.
func parseApp(reader io.Reader, key string) (app App, err error) {
	app = App{}
	_, err = toml.NewDecoder(reader).Decode(&app)
	if err != nil {
		return app, err
	}

	for _, m := range app.Maps {
		for k, p := range m.Parameters {
			p.Normalize()
			m.Parameters[k] = p
		}
	}

	app.Key = key
	return app, nil
}
