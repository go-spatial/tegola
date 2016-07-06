//tegola server
package main

import (
	"log"
	"os"

	"github.com/naoina/toml"

	"github.com/terranodo/tegola/server"
)

type Config struct {
	Server struct {
		Port string
	}
	Providers map[string]struct {
		Type     string
		Host     string
		Port     int
		Database string
		User     string
		Password string
	}
	Maps map[string]struct {
		MinZoom int
		MaxZoom int
	}
	Layers map[string]struct {
		Provider string
		Config   string
	}
}

func main() {
	//	open our config file
	f, err := os.Open("config.toml")
	if err != nil {
		log.Fatal("unable to open config file: ", err)
	}
	defer f.Close()

	//	hold parsed config from config file
	var conf Config

	//	unmarshal to our server config
	if err := toml.NewDecoder(f).Decode(&conf); err != nil {
		log.Fatal("config file error: ", err)
	}

	log.Printf("config %+v\n", conf)

	server.Init(mapServerConf(conf))

	//	TODO: move port to conifg file
	server.Start(conf.Server.Port)
}

//	map our config file to our server config
func mapServerConf(conf Config) server.Config {
	c := server.Config{
		Providers: map[string]server.Provider{},
		Maps:      map[string]server.Map{},
		Layers:    map[string]server.Layer{},
	}

	//	provider mapping
	for i := range conf.Providers {
		c.Providers[i] = server.Provider{
			Type:     conf.Providers[i].Type,
			Host:     conf.Providers[i].Host,
			Port:     conf.Providers[i].Port,
			Database: conf.Providers[i].Database,
			User:     conf.Providers[i].User,
			Password: conf.Providers[i].Password,
		}
	}

	for i := range conf.Maps {
		c.Maps[i] = server.Map{
			MinZoom: conf.Maps[i].MinZoom,
			MaxZoom: conf.Maps[i].MaxZoom,
		}
	}

	for i := range conf.Layers {
		c.Layers[i] = server.Layer{
			Provider: conf.Layers[i].Provider,
			Config:   conf.Layers[i].Config,
		}
	}

	return c
}
