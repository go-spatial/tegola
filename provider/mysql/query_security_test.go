package mysql

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
)

func TestFeatureConstructorDoesNotDumpCredentials(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(log.NewLoggerTo(&output, slog.LevelDebug))
	t.Cleanup(func() { slog.SetDefault(previous) })
	const marker = "fixture-only-sensitive-password"
	_, err := NewTileProvider(dict.Dict{"password": marker}, nil)
	if err == nil {
		t.Fatal("missing host fixture unexpectedly registered")
	}
	if strings.Contains(output.String(), marker) || strings.Contains(output.String(), "config: map[") {
		t.Fatal("constructor debug log dumped raw credentials/config")
	}
}
