package mysql

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
)

type diagnosticPanicTile struct{ provider.Tile }

func (*diagnosticPanicTile) ZXY() (slippy.Zoom, uint, uint) { panic("private tile panic") }

type diagnosticPanicSink struct{ queryDiagnosticCapture }

func (*diagnosticPanicSink) Handle(context.Context, slog.Record) error { panic("private sink panic") }

func TestDiagnosticsCannotReplaceUnknownLayerError(t *testing.T) {
	captureQueryDiagnostics(t)
	var typedNil *diagnosticPanicTile
	for _, tile := range []provider.Tile{nil, typedNil, &diagnosticPanicTile{}} {
		p := &Provider{}
		err := p.TileFeatures(context.Background(), "missing", tile, nil, nil)
		var unknown ErrUnknownLayer
		if !errors.As(err, &unknown) {
			t.Fatalf("diagnostics replaced unknown-layer error: %v", err)
		}
	}
}

func TestTileQueryGoexitPreserved(t *testing.T) {
	captureQueryDiagnostics(t)
	p := diagnosticProvider(t)
	done := make(chan bool, 1)
	go func() {
		returned := false
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- true
				return
			}
			done <- returned
		}()
		_ = p.TileFeatures(context.Background(), "diagnostics",
			provider.NewTile(0, 0, 0, 0, tegola.WebMercator), nil,
			func(*provider.Feature) error { runtime.Goexit(); return nil })
		returned = true
	}()
	if <-done {
		t.Fatal("Goexit was replaced by panic or normal completion")
	}
}

func TestTileQueryPanicNilCompatibility(t *testing.T) {
	const marker = "TEGOLA_TEST_DIAGNOSTIC_PANICNIL"
	if os.Getenv(marker) == "child" {
		captureQueryDiagnostics(t)
		p := diagnosticProvider(t)
		returned := false
		func() {
			defer func() {
				_ = recover()
				if returned {
					t.Fatal("panic(nil) was converted into normal completion")
				}
			}()
			_ = p.TileFeatures(context.Background(), "diagnostics",
				provider.NewTile(0, 0, 0, 0, tegola.WebMercator), nil,
				func(*provider.Feature) error { panic(nil) })
			returned = true
		}()
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestTileQueryPanicNilCompatibility$")
	command.Env = append(os.Environ(), marker+"=child", "GODEBUG=panicnil=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("panicnil compatibility subprocess failed: %v\n%s", err, output)
	}
}

func TestDiagnosticsSinkPanicIsIsolated(t *testing.T) {
	previous := slog.Default()
	slog.SetDefault(slog.New(&diagnosticPanicSink{}))
	t.Cleanup(func() { slog.SetDefault(previous) })
	d := tileQueryDiagnostics{started: time.Now(), phaseStarted: time.Now(), phase: "prepare"}
	d.finish(nil, "layer", &diagnosticPanicTile{}, 1, errors.New("business error"))
}

func TestInspectionGeometryCellLimit(t *testing.T) {
	// Exact boundary is accepted; the sentinel byte is rejected before decode.
	value := make([]byte, 64<<20+1)
	if err := inspectionGeometrySize(value[:len(value)-1]); err != nil {
		t.Fatal(err)
	}
	if err := inspectionGeometrySize(value); err == nil {
		t.Fatal("oversized sample accepted")
	}
}
