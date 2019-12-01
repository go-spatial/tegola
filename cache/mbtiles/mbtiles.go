package mbtiles

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
)

var (
	ErrMissingBasepath = errors.New("mbtilescache: missing required param 'basepath'")
)

const CacheType = "mbtiles"

const (
	ConfigKeyBasepath = "basepath"
	ConfigKeyMaxZoom  = "max_zoom"
	ConfigKeyMinZoom  = "min_zoom"
)

func init() {
	cache.Register(CacheType, New)
}

//Cache hold the cache configuration
type Cache struct {
	Basepath string
	// MinZoom determines the min zoom the cache to persist. Before this
	// zoom, cache Set() calls will be ignored.
	MinZoom uint
	// MaxZoom determines the max zoom the cache to persist. Beyond this
	// zoom, cache Set() calls will be ignored. This is useful if the cache
	// should not be leveraged for higher zooms when data changes often.
	MaxZoom uint
	// reference to the database connections
	DBList map[string]*sql.DB
	// for managing current access to the DBList
	sync.RWMutex
}

//Get reads a z,x,y entry from the cache and returns the contents
// if there is a hit. the second argument denotes a hit or miss
// so the consumer does not need to sniff errors for cache read misses
func (fc *Cache) Get(key *cache.Key) ([]byte, bool, error) {
	db, err := fc.openOrCreateDB(key.MapName, key.LayerName)
	if err != nil {
		return nil, false, err
	}
	yCorr := (1 << key.Z) - 1 - key.Y
	var data []byte
	err = db.QueryRow("SELECT tile_data FROM tiles WHERE zoom_level = ? AND tile_column = ? AND tile_row = ?", key.Z, key.X, yCorr).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

//Set save a z,x,y entry in the cache
func (fc *Cache) Set(key *cache.Key, val []byte) error {
	// check for maxzoom and minzoom
	if key.Z > fc.MaxZoom || key.Z < fc.MinZoom {
		return nil
	}

	db, err := fc.openOrCreateDB(key.MapName, key.LayerName)
	if err != nil {
		return err
	}
	yCorr := (1 << key.Z) - 1 - key.Y
	_, err = db.Exec("INSERT OR REPLACE INTO tiles (zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?)", key.Z, key.X, yCorr, val)
	return err
}

//Purge clear a z,x,y entry from the cache
func (fc *Cache) Purge(key *cache.Key) error {
	db, err := fc.openOrCreateDB(key.MapName, key.LayerName)
	if err != nil {
		return err
	}
	yCorr := (1 << key.Z) - 1 - key.Y
	_, err = db.Exec("DELETE FROM tiles WHERE zoom_level = ? AND tile_column = ? AND tile_row = ?", key.Z, key.X, yCorr)
	return err
}

func (fc *Cache) openOrCreateDB(mapName, layerName string) (*sql.DB, error) {
	if mapName == "" {
		mapName = "default"
	}
	fileName := mapName
	if layerName != "" {
		fileName += "-" + layerName
	}

	fc.Lock()
	defer fc.Unlock()
	//Look for open connection in DBList
	db, ok := fc.DBList[fileName]
	if ok {
		return db, nil
	}

	//Connection is not ready we need one
	file := filepath.Join(fc.Basepath, fileName+".mbtiles")

	//Check if file exist prior to init
	_, err := os.Stat(file)
	dbNeedInit := os.IsNotExist(err)

	db, err = sql.Open("sqlite3", file)
	if err != nil {
		return nil, err
	}
	if dbNeedInit {
		for _, initSt := range []string{
			"CREATE TABLE metadata (name text, value text)",
			"CREATE UNIQUE INDEX metadata_name on metadata (name)",
			"CREATE TABLE tiles (zoom_level integer, tile_column integer, tile_row integer, tile_data blob)",
			"CREATE UNIQUE INDEX tile_index on tiles (zoom_level, tile_column, tile_row)",
			//"CREATE TABLE grids (zoom_level integer, tile_column integer, tile_row integer, grid blob)",
			//"CREATE TABLE grid_data (zoom_level integer, tile_column integer, tile_row integer, key_name text, key_json text)",
		} {
			_, err := db.Exec(initSt)
			if err != nil {
				return nil, err
			}
		}
		//TODO find better storage in sqlite + use views
	}

	// lookup our Map
	m, err := atlas.GetMap(mapName)
	if err != nil {
		log.Printf("mbtilescache: fail to retrieve map '%s' details: %v", mapName, err)
	} else {
		tileJSON := m.GetBaseTileJSON()

		bJSON, err := json.Marshal(tileJSON)
		if err != nil {
			log.Printf("mbtilescache: fail to encode vector layers: %s", err)
		} else {
			for metaName, metaValue := range map[string]string{
				"name":        m.Name,
				"description": "Tegola Cache Tiles",
				"format":      tileJSON.Format,
				"bounds":      BoundsToString(m.Bounds.Extent()),
				"center":      fmt.Sprintf("%f,%f,%f", m.Center[0], m.Center[1], m.Center[2]),
				"minzoom":     fmt.Sprintf("%d", Max(fc.MinZoom, tileJSON.MinZoom)),
				"maxzoom":     fmt.Sprintf("%d", Min(fc.MaxZoom, tileJSON.MaxZoom)),
				"json":        string(bJSON), //TODO only output selected layer if layerName is set
				"version":     tileJSON.Version,
				"attribution": m.Attribution,
				"type":        "overlay",
			} {

				_, err = db.Exec("INSERT OR REPLACE INTO metadata (name, value) VALUES (?, ?)", metaName, metaValue)
				if err != nil {
					log.Printf("mbtilescache: fail to write metadata: %s", err)
				}
			}
		}
	}

	//Store connection
	fc.DBList[fileName] = db
	return db, nil
}

//BoundsToString return a string representation of cache bounds
func BoundsToString(b [4]float64) string {
	return fmt.Sprintf("%f,%f,%f,%f", b[0], b[1], b[2], b[3])
}

// Max returns the larger of x or y.
func Max(x, y uint) uint {
	if x < y {
		return y
	}
	return x
}

// Min returns the smaller of x or y.
func Min(x, y uint) uint {
	if x > y {
		return y
	}
	return x
}
