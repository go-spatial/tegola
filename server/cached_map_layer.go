package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"

	vectorTile "github.com/alexeydott/geom/encoding/mvt/vector_tile"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"google.golang.org/protobuf/proto"
)

// cachedMapLayer reuses a seeded whole-map tile for a layer endpoint. Geometry
// commands and feature attributes are retained verbatim; no provider is called.
// Do not persist a derived tile: that would restart its TTL and could outlive
// the map tile or overwrite an independently regenerated layer tile.
func cachedMapLayer(ctx context.Context, a *atlas.Atlas, c cache.Interface, key *cache.Key) ([]byte, bool, error) {
	if key.LayerName == "" {
		return nil, false, nil
	}
	m, err := a.Map(key.MapName)
	if err != nil || len(m.Params) != 0 {
		return nil, false, nil
	}
	active := m.FilterLayersByZoom(slippy.Zoom(key.Z))
	m = active.FilterLayersByName(key.LayerName)
	if len(m.Layers) == 0 {
		return nil, false, nil
	}
	inside, err := tileWithinMapBounds(&m, slippy.Tile{Z: slippy.Zoom(key.Z), X: key.X, Y: key.Y})
	if err != nil || !inside {
		return nil, false, err
	}
	// A provider-name alias must not expose features from another configured
	// layer sharing its MVT name: those features cannot be separated afterwards.
	for _, selected := range m.Layers {
		for _, other := range active.Layers {
			if selected.MVTName() == other.MVTName() && other.Name != key.LayerName && other.ProviderLayerName != key.LayerName {
				return nil, false, nil
			}
		}
	}
	mapKey := *key
	mapKey.LayerName = ""
	compressed, hit, err := c.Get(ctx, &mapKey)
	if err != nil || !hit {
		return nil, false, err
	}
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, false, err
	}
	raw, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		return nil, false, err
	}
	var tile vectorTile.Tile
	if err = proto.Unmarshal(raw, &tile); err != nil {
		return nil, false, err
	}
	names := make(map[string]bool, len(m.Layers))
	for _, l := range m.Layers {
		names[l.MVTName()] = true
	}
	selected := tile.Layers[:0]
	for _, l := range tile.Layers {
		if names[l.GetName()] {
			selected = append(selected, l)
		}
	}
	tile.Layers = selected
	raw, err = proto.Marshal(&tile)
	if err != nil {
		return nil, false, err
	}
	var output bytes.Buffer
	w := gzip.NewWriter(&output)
	if _, err = w.Write(raw); err != nil {
		return nil, false, err
	}
	if err = w.Close(); err != nil {
		return nil, false, err
	}
	return output.Bytes(), true, nil
}
