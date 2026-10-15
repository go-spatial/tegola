package atlas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	vectorTile "github.com/alexeydott/geom/encoding/mvt/vector_tile"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/provider"
	"google.golang.org/protobuf/proto"
)

type deadlineTileProvider struct{ errorTileProvider }

func (deadlineTileProvider) TileFeatures(ctx context.Context, _ string, _ provider.Tile, _ provider.Params, _ func(*provider.Feature) error) error {
	<-ctx.Done()
	return ctx.Err()
}

type diagnosticTileProvider struct {
	errorTileProvider
	run func(context.Context, func(*provider.Feature) error) error
}

func (p diagnosticTileProvider) TileFeatures(ctx context.Context, _ string, _ provider.Tile, _ provider.Params, callback func(*provider.Feature) error) error {
	return p.run(ctx, callback)
}

type timingPanicHandler struct{ slog.Handler }

func (h timingPanicHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "tile layer timing" {
		panic("diagnostic handler failed")
	}
	return h.Handler.Handle(ctx, record)
}

type panicDiagnosticError struct{}

func (panicDiagnosticError) Error() string { panic("error formatting failed") }

func TestLayerDiagnosticErrorMethodPanicContained(t *testing.T) {
	logLayerTiming("map", Layer{}, slippy.Tile{}, time.Millisecond, 0, panicDiagnosticError{})
}

func TestLayerDiagnosticNoRawErrorAndCancellationLevel(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	secret := "SELECT private_value WHERE password = sensitive"
	logLayerTiming("map", Layer{Name: "layer"}, slippy.Tile{}, time.Millisecond, 0, errors.New(secret))
	logLayerTiming("map", Layer{Name: "layer"}, slippy.Tile{}, time.Second, 0, context.Canceled)
	if strings.Contains(output.String(), secret) {
		t.Fatal("raw error leaked into timing diagnostic")
	}
	var records []map[string]any
	for _, line := range bytes.Split(output.Bytes(), []byte("\n")) {
		var record map[string]any
		if json.Unmarshal(line, &record) == nil {
			records = append(records, record)
		}
	}
	if len(records) != 2 || records[0]["outcome"] != "error" || records[0]["level"] != "WARN" || records[1]["outcome"] != "canceled" || records[1]["level"] != "DEBUG" {
		t.Fatalf("diagnostic policy changed: %v", records)
	}
}

func TestLayerDiagnosticCountsAndTileBytes(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	base := slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(base))
	m := NewWebMercatorMap("count-map")
	m.Layers = []Layer{{Name: "points", ProviderLayerName: "source", Provider: diagnosticTileProvider{run: func(_ context.Context, callback func(*provider.Feature) error) error {
		if err := callback(&provider.Feature{ID: 1, SRID: 3857, Geometry: geom.Collection{}}); err != nil {
			return err
		}
		return callback(&provider.Feature{ID: 2, SRID: 3857, Geometry: geom.Point{0, 0}, Tags: map[string]any{"name": "point"}})
	}}}}
	before, err := m.encodeMVTTile(context.Background(), slippy.Tile{Z: 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var tile vectorTile.Tile
	if err := proto.Unmarshal(before, &tile); err != nil {
		t.Fatal(err)
	}
	if len(tile.Layers) != 1 || len(tile.Layers[0].Features) != 1 {
		t.Fatalf("unexpected encoded features: %v", &tile)
	}
	found := false
	for _, line := range bytes.Split(output.Bytes(), []byte("\n")) {
		var record map[string]any
		if json.Unmarshal(line, &record) == nil && record["msg"] == "tile layer timing" {
			found = true
			if record["features_received"] != float64(2) || record["outcome"] != "ok" {
				t.Fatalf("callback count changed: %v", record)
			}
		}
	}
	if !found {
		t.Fatal("missing completion record")
	}
	slog.SetDefault(slog.New(timingPanicHandler{Handler: base}))
	after, err := m.encodeMVTTile(context.Background(), slippy.Tile{Z: 0}, nil)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("diagnostic panic changed tile result: %v", err)
	}
}

func TestLayerDiagnosticPreservesBusinessPanicAndError(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(timingPanicHandler{Handler: slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})}))
	t.Cleanup(func() { slog.SetDefault(previous) })
	sentinel := errors.New("business-error")
	for _, test := range []struct {
		name string
		run  func(context.Context, func(*provider.Feature) error) error
	}{
		{name: "error", run: func(context.Context, func(*provider.Feature) error) error { return sentinel }},
		{name: "panic", run: func(context.Context, func(*provider.Feature) error) error { panic("business-panic") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := NewWebMercatorMap("error-map")
			m.Layers = []Layer{{Name: "points", ProviderLayerName: "source", Provider: diagnosticTileProvider{run: test.run}}}
			_, err := m.encodeMVTTile(context.Background(), slippy.Tile{Z: 0}, nil)
			if err == nil || test.name == "error" && !errors.Is(err, sentinel) {
				t.Fatalf("business failure lost: %v", err)
			}
			if test.name == "panic" && !strings.Contains(err.Error(), "business-panic") {
				t.Fatalf("diagnostic replaced business panic: %v", err)
			}
		})
	}
}

func TestLayerDeadlineDiagnostic(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	m := NewWebMercatorMap("test-map")
	m.Layers = []Layer{{Name: "public-layer", ProviderLayerName: "source-layer", Provider: deadlineTileProvider{}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := m.encodeMVTTile(ctx, slippy.Tile{Z: 15, X: 19782, Y: 10323}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error identity changed: %v", err)
	}
	for _, line := range bytes.Split(output.Bytes(), []byte("\n")) {
		var record map[string]any
		if json.Unmarshal(line, &record) != nil || record["msg"] != "tile layer timing" {
			continue
		}
		if record["outcome"] != "deadline" || record["level"] != "WARN" || record["map"] != "test-map" || record["provider_layer"] != "source-layer" || record["x"] != float64(19782) {
			t.Fatalf("unexpected diagnostic: %v", record)
		}
		return
	}
	t.Fatal("missing layer deadline diagnostic")
}
