package provider

import "github.com/go-spatial/tegola/internal/env"

// A Map represents a map in the Tegola Config file.
type Map struct {
	Name        env.String       `toml:"name"`
	Attribution env.String       `toml:"attribution"`
	Bounds      []env.Float      `toml:"bounds"`
	Center      [3]env.Float     `toml:"center"`
	Layers      []MapLayer       `toml:"layers"`
	Parameters  []QueryParameter `toml:"params"`
	TileBuffer  *env.Int         `toml:"tile_buffer"`
}
