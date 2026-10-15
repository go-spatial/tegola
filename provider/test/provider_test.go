package test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func TestMVTForLayersCanned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tp := range []*TileProvider{nil, {}, {MVTTile: []byte{0xff, 1, 2}}} {
		got, err := tp.MVTForLayers(ctx, nil, nil, []provider.Layer{{Name: "unregistered", MVTName: "alias"}})
		if err != nil {
			t.Fatalf("legacy canned mode returned an error: %v", err)
		}
		var want []byte
		if tp != nil {
			want = tp.MVTTile
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("canned bytes = %v, want %v", got, want)
		}
	}
}

func TestMVTForLayersCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tile := provider.NewTile(3, 2, 1, 64, 3857)
	params := provider.Params{"!REGION!": {Value: "west"}}
	layers := []provider.Layer{{Name: "source", MVTName: "alias"}, {Name: "second"}}
	wantErr := errors.New("injected MVT failure")
	calls := 0
	tp := &TileProvider{
		MVTTile: []byte("ignored"),
		MVTForLayersFunc: func(gotCtx context.Context, gotTile provider.Tile, gotParams provider.Params, gotLayers []provider.Layer) ([]byte, error) {
			calls++
			if gotCtx != ctx || gotTile != tile || !reflect.DeepEqual(gotParams, params) || !reflect.DeepEqual(gotLayers, layers) {
				t.Fatal("callback did not receive the original request")
			}
			if err := gotCtx.Err(); err != nil {
				return nil, err
			}
			return []byte("callback"), wantErr
		},
	}
	got, err := tp.MVTForLayers(ctx, tile, params, layers)
	if string(got) != "callback" || !errors.Is(err, wantErr) || calls != 1 {
		t.Fatalf("callback result = %q, %v; calls = %d", got, err, calls)
	}
	cancel()
	if _, err := tp.MVTForLayers(ctx, tile, params, layers); !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatalf("callback cancellation = %v; calls = %d", err, calls)
	}
}

func TestNewMVTTileProviderFixture(t *testing.T) {
	t.Cleanup(Cleanup)
	path := filepath.Join(t.TempDir(), "tile.mvt")
	want := []byte{0xff, 1, 2}
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	tp, err := NewMVTTileProvider(dict.Dict{"test_file": path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tp.MVTForLayers(context.Background(), nil, nil, nil)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("fixture result = %v, %v", got, err)
	}
	// On Windows removal also verifies the loader released the file handle.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMVTTileProvider(dict.Dict{"test_file": path}, nil); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing fixture error = %v", err)
	}
	if _, err := NewMVTTileProvider(dict.Dict{}, nil); err == nil {
		t.Fatal("missing test_file config must fail")
	}
	if _, err := NewMVTTileProvider(nil, nil); err != nil {
		t.Fatalf("nil config must keep the empty canned provider: %v", err)
	}
}
