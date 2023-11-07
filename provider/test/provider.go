package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/provider"

	"github.com/go-spatial/tegola/dict"
)

const Name = "test"

var (
	lock     sync.Mutex
	Count    int
	MVTCount int
)

func init() {
	provider.Register(provider.TypeStd.Prefix()+Name, NewTileProvider)
	provider.MVTRegister(provider.TypeMvt.Prefix()+Name, NewMVTTileProvider)
}

// NewTileProvider setups a test provider. there are not currently any config params supported
func NewTileProvider(config dict.Dicter, maps []provider.Map) (provider.Tiler, error) {
	lock.Lock()
	Count++
	lock.Unlock()
	return &TileProvider{}, nil
}

// NewMVTTileProvider setups a test provider for mvt tiles providers. The only supported parameter is
// "test_file", which should point to a mvt tile file to return for MVTForLayers
func NewMVTTileProvider(config dict.Dicter, maps []provider.Map) (provider.MVTTiler, error) {
	lock.Lock()
	MVTCount++
	lock.Unlock()
	mvtTile := []byte{}
	if config != nil {
		path, err := config.String("test_file", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get test_file key: %w", err)
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open test_file: %w", err)
		}
		mvtTile, err = ioutil.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read test_file: %w", err)
		}
	}
	return &TileProvider{
		MVTTile: mvtTile,
	}, nil
}

// TileProvider mocks out a tile provider
type TileProvider struct {
	MVTTile []byte
}

// Layers returns the configured layers, there is always only one "test-layer"
func (tp *TileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: geom.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

// TileFeatures always returns a feature with a polygon outlining the tile's Extent (not Buffered Extent)
func (tp *TileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, queryParams provider.Params, fn func(f *provider.Feature) error) error {
	// get tile bounding box
	ext, srid := t.Extent()

	debugTileOutline := provider.Feature{
		ID:       0,
		Geometry: ext.AsPolygon(),
		SRID:     srid,
		Tags: map[string]interface{}{
			"type": "debug_buffer_outline",
		},
	}

	return fn(&debugTileOutline)
}

// MVTForLayers mocks out MVTForLayers by just returning the MVTTile bytes, this will never error
func (tp *TileProvider) MVTForLayers(ctx context.Context, _ provider.Tile, _ provider.Params, _ []provider.Layer) ([]byte, error) {
	// TODO(gdey): fill this out.
	if tp == nil {
		return nil, nil
	}
	return tp.MVTTile, nil
}

// Cleanup cleans up the test provider.
func (tp *TileProvider) Cleanup() error {
	lock.Lock()
	if tp.MVTTile == nil {
		Count--
	} else {
		MVTCount--
	}
	lock.Unlock()
	return nil
}
