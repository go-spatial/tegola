package cache

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/provider"
)

// TestDoWorkSkipsMapsWithCustomParams ensures maps with custom parameters are
// excluded from caching entirely: they are warned about and skipped, while
// plain maps are still cached (part13 P6-25).
func TestDoWorkSkipsMapsWithCustomParams(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	maps := []atlas.Map{
		{Name: "plain"},
		{Name: "paramy", Params: []provider.QueryParameter{{Name: "foo", Token: "!FOO!"}}},
	}

	var mu sync.Mutex
	var got []string
	worker := func(_ context.Context, mt MapTile) error {
		mu.Lock()
		got = append(got, mt.MapName)
		mu.Unlock()
		return nil
	}

	tileChannel := &TileChannel{channel: make(chan slippy.Tile)}
	go func() {
		tileChannel.channel <- slippy.Tile{}
		tileChannel.Close()
	}()

	if err := doWork(context.Background(), tileChannel, maps, 1, worker); err != nil {
		t.Fatalf("doWork returned error: %v", err)
	}

	if len(got) != 1 || got[0] != "plain" {
		t.Errorf("expected only the plain map to be cached, got maps processed: %v", got)
	}
	if !strings.Contains(buf.String(), "caching is disabled for map paramy") {
		t.Errorf("expected a warning about the parameterized map, got log output: %q", buf.String())
	}
	if strings.Contains(buf.String(), "caching is disabled for map plain") {
		t.Errorf("did not expect a warning for the plain map, got log output: %q", buf.String())
	}
}
