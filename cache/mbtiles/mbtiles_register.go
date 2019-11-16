// +build cgo

package mbtiles

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/cmd"
)

// New instantiates a Cache. The config expects the following params:
//
// 	basepath (string): a path to where the cache will be written
// 	max_zoom (int): max zoom to use the cache. beyond this zoom cache Set() calls will be ignored
// 	min_zoom (int): min zoom to use the cache. before this zoom cache Set() calls will be ignored
// 	bounds (string): bounds to use the cache. outside this bounds cache Set() calls will be ignored
//
func New(config dict.Dicter) (cache.Interface, error) {
	var err error

	// new filecache
	fc := Cache{}

	defaultMaxZoom := uint(tegola.MaxZ)
	fc.MaxZoom, err = config.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	defaultMinZoom := uint(0)
	fc.MinZoom, err = config.Uint(ConfigKeyMinZoom, &defaultMinZoom)
	if err != nil {
		return nil, err
	}

	defaultBounds := EarthBounds.String()
	cfgBounds, err := config.String(ConfigKeyBounds, &defaultBounds)
	if err != nil {
		return nil, err
	}

	// validate and set bounds flag
	boundsParts := strings.Split(strings.TrimSpace(cfgBounds), ",")
	if len(boundsParts) != 4 {
		return nil, fmt.Errorf("mbtilescache: invalid value for bounds (%v). expecting minx, miny, maxx, maxy", cfgBounds)
	}

	var ok bool

	if fc.Bounds[0], ok = cmd.IsValidLngString(boundsParts[0]); !ok {
		return nil, fmt.Errorf("mbtilescache: invalid lng value(%v) for bounds (%v)", boundsParts[0], cfgBounds)
	}
	if fc.Bounds[1], ok = cmd.IsValidLatString(boundsParts[1]); !ok {
		return nil, fmt.Errorf("mbtilescache: invalid lat value(%v) for bounds (%v)", boundsParts[1], cfgBounds)
	}
	if fc.Bounds[2], ok = cmd.IsValidLngString(boundsParts[2]); !ok {
		return nil, fmt.Errorf("mbtilescache: invalid lng value(%v) for bounds (%v)", boundsParts[2], cfgBounds)
	}
	if fc.Bounds[3], ok = cmd.IsValidLatString(boundsParts[3]); !ok {
		return nil, fmt.Errorf("mbtilescache: invalid lat value(%v) for bounds (%v)", boundsParts[3], cfgBounds)
	}

	fc.Basepath, err = config.String(ConfigKeyBasepath, nil)
	if err != nil {
		return nil, ErrMissingBasepath
	}

	if fc.Basepath == "" {
		return nil, ErrMissingBasepath
	}

	// make our basepath if it does not exist
	if err = os.MkdirAll(fc.Basepath, os.ModePerm); err != nil {
		return nil, err
	}

	fc.DBList = make(map[string]*sql.DB)

	return &fc, nil
}
