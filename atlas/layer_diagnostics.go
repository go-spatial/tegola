package atlas

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/internal/log"
)

// Layer time includes provider work and feature callbacks, but not final tile serialization.
func logLayerTiming(mapName string, layer Layer, tile slippy.Tile, elapsed time.Duration, features uint64, err error) {
	// Diagnostics run after the layer's business panic guard. A custom logger or
	// error method must not introduce a second panic or change the layer result.
	defer func() { _ = recover() }()

	outcome := "ok"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "deadline"
	case errors.Is(err, context.Canceled), err != nil && strings.Contains(err.Error(), "operation was canceled"):
		outcome = "canceled"
	case err != nil:
		outcome = "error"
	}
	attrs := []any{
		"map", mapName, "layer", layer.MVTName(), "provider_layer", layer.ProviderLayerName,
		"z", tile.Z, "x", tile.X, "y", tile.Y,
		"elapsed_ms", float64(elapsed) / float64(time.Millisecond),
		"features_received", features, "outcome", outcome,
	}
	if outcome != "canceled" && (err != nil || elapsed >= time.Second) {
		log.Logger().Warn("tile layer timing", attrs...)
		return
	}
	log.Logger().Debug("tile layer timing", attrs...)
}
