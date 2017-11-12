package config

import (
	"flag"
	"fmt"
	"strings"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/config"
	"github.com/terranodo/tegola/provider/postgis"
)

type C struct {
	File       string
	Layer      string
	Coords     [3]int
	IsolateGeo int64
	ToClip     bool
	Basedir    string
	Extent     int64

	provider *ProviderLayer
}

func (cfg *C) InitFlags() {
	const (
		defaultConfigFile  = "config.toml"
		usageConfigFile    = "The config file for tegola."
		usageMapName       = "The map name to use. If one isn't provided the first map is used."
		usageProviderLayer = "The Provider and the Layer to use — must be a postgis provider. “$provider.$layer” [required]"
	)
	flag.StringVar(&(cfg.File), "config", defaultConfigFile, usageConfigFile)
	flag.StringVar(&(cfg.File), "c", defaultConfigFile, usageConfigFile+" (shorthand)")
	flag.StringVar(&(cfg.Layer), "provider", "", usageProviderLayer)
	flag.StringVar(&(cfg.Layer), "p", "", usageProviderLayer+" (shorthand)")
	flag.IntVar(&(cfg.Coords[0]), "z", 0, "The Z coord")
	flag.IntVar(&(cfg.Coords[1]), "x", 0, "The X coord")
	flag.IntVar(&(cfg.Coords[2]), "y", 0, "The Y coord")
	flag.Int64Var(&(cfg.IsolateGeo), "g", -1, "Only grab the feature with the geoid. -1 means all of them.")
}

func (cfg *C) Tile() tegola.Tile {
	return tegola.Tile{
		X: cfg.Coords[1],
		Y: cfg.Coords[2],
		Z: cfg.Coords[0],
	}
}

func (cfg *C) X() int {
	return cfg.Coords[1]
}
func (cfg *C) Y() int {
	return cfg.Coords[2]
}
func (cfg *C) Z() int {
	return cfg.Coords[0]
}

func (cfg *C) LoadProvider() (pl ProviderLayer, err error) {
	tcfg, err := config.Load(cfg.File)
	if err != nil {
		return pl, err
	}
	if len(tcfg.Providers) == 0 {
		return pl, fmt.Errorf("No Providers defined in config.")
	}

	providerName, layerName := splitProviderLayer(cfg.Layer)
	if providerName == "" {
		// Need to look up the provider
		for _, p := range tcfg.Providers {
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
				Name:     layerName,
				Config:   p,
				Provider: provider,
			}, nil
		}
	}

	for _, p := range tcfg.Providers {
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
			Name:     layerName,
			Config:   p,
			Provider: provider,
		}, nil
	}
	cfg.provider = &pl

	return pl, fmt.Errorf("Could not find provider(%v).", providerName)
}

func (cfg *C) initProvider() error {
	if cfg.provider != nil {
		return nil
	}
	pl, err := cfg.LoadProvider()
	if err != nil {
		return err
	}
	cfg.provider = &pl
	return nil
}

func (cfg *C) Provider() (p postgis.Provider, err error) {
	cfg.initProvider()
	return cfg.provider.Provider, nil
}

func (cfg *C) ProvderConfig() (c map[string]interface{}, err error) {
	if err = cfg.initProvider(); err != nil {
		return c, err
	}
	return cfg.provider.Config, nil
}
func (cfg *C) ProviderName() (string, error) {
	if err := cfg.initProvider(); err != nil {
		return "", err
	}
	return cfg.provider.Name, nil
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
	Name     string
	Config   map[string]interface{}
	Provider postgis.Provider
}
