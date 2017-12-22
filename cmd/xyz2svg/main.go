package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"os"

	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/config"
	"github.com/terranodo/tegola/draw/svg"
	"github.com/terranodo/tegola/maths/validate"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/provider/postgis"
	"github.com/terranodo/tegola/wkb"
)

var configStruct struct {
	ConfigFile  string
	Layer       string
	Coords      [3]int
	IsolateGeo  int64
	ToClip      bool
	KeepNilClip bool
}

func splitProviderLayer(providerLayer string) (provider, layer string) {
	parts := strings.SplitN(providerLayer, ".", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}

type ProviderLayer struct {
	name     string
	config   map[string]interface{}
	provider postgis.Provider
}

func LoadProvider(configFile string, providerlayer string) (pl ProviderLayer, err error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		return pl, err
	}
	if len(cfg.Providers) == 0 {
		return pl, fmt.Errorf("No Providers defined in config.")
	}

	providerName, layerName := splitProviderLayer(providerlayer)
	if providerName == "" {
		// Need to look up the provider
		for _, p := range cfg.Providers {
			t, _ := p["type"].(string)
			if t != "postgis" {
				continue
			}
			mvtprovider, err := postgis.NewProvider(p)
			if err != nil {
				return pl, err
			}
			provider, _ := mvtprovider.(postgis.Provider)
			return ProviderLayer{
				name:     layerName,
				config:   p,
				provider: provider,
			}, nil
		}
	}

	for _, p := range cfg.Providers {
		t, _ := p["type"].(string)
		if t != "postgis" {
			continue
		}
		name, _ := p["name"].(string)
		if name != providerName {
			continue
		}
		mvtprovider, err := postgis.NewProvider(p)
		if err != nil {
			return pl, err
		}
		provider, _ := mvtprovider.(postgis.Provider)
		return ProviderLayer{
			name:     layerName,
			config:   p,
			provider: provider,
		}, nil
	}

	return pl, fmt.Errorf("Could not find provider(%v).", providerName)
}

func init() {
	const (
		defaultConfigFile  = "config.toml"
		usageConfigFile    = "The config file for tegola."
		usageMapName       = "The map name to use. If one isn't provided the first map is used."
		usageProviderLayer = "The Provider and the Layer to use — must be a postgis provider. “$provider.$layer” [required]"
	)
	flag.StringVar(&configStruct.ConfigFile, "config", defaultConfigFile, usageConfigFile)
	flag.StringVar(&configStruct.ConfigFile, "c", defaultConfigFile, usageConfigFile+" (shorthand)")
	flag.StringVar(&configStruct.Layer, "provider", "", usageProviderLayer)
	flag.StringVar(&configStruct.Layer, "p", "", usageProviderLayer+" (shorthand)")
	flag.IntVar(&(configStruct.Coords[0]), "z", 0, "The Z coord")
	flag.IntVar(&(configStruct.Coords[1]), "x", 0, "The X coord")
	flag.IntVar(&(configStruct.Coords[2]), "y", 0, "The Y coord")
	flag.Int64Var(&configStruct.IsolateGeo, "g", -1, "Only grab the geo described. -1 means all of them.")
	flag.BoolVar(&configStruct.ToClip, "clip", false, "Scale image to clipping region.")
	flag.BoolVar(&configStruct.KeepNilClip, "all", false, "Generate images for features that are clipped to nil.")
}

func DrawGeometries() {
	provider, err := LoadProvider(configStruct.ConfigFile, configStruct.Layer)
	if err != nil {
		panic(err)
	}

	tile := tegola.Tile{
		X: configStruct.Coords[1],
		Y: configStruct.Coords[2],
		Z: configStruct.Coords[0],
	}

	p := provider.provider
	baseDir := fmt.Sprintf("svg_files/z%v_x%v_y%v", tile.Z, tile.X, tile.Y)
	if err := os.MkdirAll(baseDir, 0711); err != nil {
		panic(err)
	}
	count := 0

	skipped := []uint64{}
	if err := p.ForEachFeature(context.Background(), provider.name, tile, func(layer postgis.Layer, gid uint64, geom wkb.Geometry, tags map[string]interface{}) error {
		if configStruct.IsolateGeo != -1 && configStruct.IsolateGeo != int64(gid) {
			return nil
		}
		count++
		cursor := mvt.NewCursor(tile.BoundingBox(), 4096)

		g := cursor.ScaleGeo(geom)
		//log.Println("Tolerence:", tile.ZEpislon())
		sg := mvt.SimplifyGeometry(g, tile.ZEpislon(), true)
		vg, err := validate.CleanGeometry(context.Background(), sg, 4096)
		if err != nil {
			return err
		}

		mm := svg.MinMax{0, 0, 4096, 4096}
		mm.ExpandBy(100)
		mm1 := svg.MinMax{4096, 4096, 0, 0}
		mm1.OfGeometry(g)
		mm1.ExpandBy(100)
		if !configStruct.ToClip {
			mm = mm1
		}

		canvas := &svg.Canvas{
			Board:  mm,
			Region: svg.MinMax{0, 0, 4096, 4096},
		}

		path := filepath.Join(baseDir, layer.Name())

		if err = os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}

		filename := fmt.Sprintf("geo_%v.svg", gid)
		path = filepath.Join(path, filename)

		log.Println(path)
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		log.Printf("Creating Geo: %v\tminmax: %v\n", gid, mm)
		canvas.Init(file, 1440, 900, false)
		log.Println("\tDrawing original version.")
		canvas.Commentf("MinMax: %v\n", mm1)
		canvas.DrawGeometry(g, fmt.Sprintf("%v_scaled", gid), "fill-rule:evenodd; fill:yellow;opacity:1", "fill:black", false)
		canvas.DrawRegion(true)
		log.Println("\tDrawing simplified version.")
		canvas.DrawGeometry(sg, fmt.Sprintf("%v_simplifed", gid), "fill-rule:evenodd; fill:green;opacity:0.5", "fill:green;opacity:0.5", false)

		log.Println("\tDrawing clipped version.")
		canvas.DrawGeometry(vg, fmt.Sprintf("clipped_%v", gid), "fill-rule:evenodd; fill:green;opacity:0.5", "fill:green;opacity:0.5", false)
		canvas.End()
		return nil
	}); err != nil {
		panic(err)
	}

	// Writes out the initial svg starting tag and draws the grid.
	fmt.Println("// Number of geometries found:", count)
	fmt.Printf("// Skipped %v \n", len(skipped))
	fmt.Printf("// Created: %v \n", count-len(skipped))
}

func main() {
	flag.Parse()
	DrawGeometries()
}
