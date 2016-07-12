//tegola server
package main

import (
	"log"
	"os"

	"github.com/naoina/toml"

	"github.com/terranodo/tegola/server"
)

type Config struct {
	Webserver struct {
		Port string
	}
	Providers []struct {
		Name     string
		Type     string
		Host     string
		Port     uint16
		Database string
		User     string
		Password string
	}
	Maps []struct {
		Name    string
		MinZoom int
		MaxZoom int
		layer   string
	}
	Layers []struct {
		Name      string
		Provider  string
		TableName string
		GeomFiled string
		FIDField  string
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

	server.Start(conf.Webserver.Port)
}

//	map our config file to our server config
func mapServerConf(conf Config) server.Config {
	c := server.Config{
		Providers: map[string]server.Provider{},
		Maps:      map[string]server.Map{},
		Layers:    map[string]server.Layer{},
	}

	//	provider mapping
	for _, provider := range conf.Providers {
		c.Providers[provider.Name] = server.Provider{
			Type:     provider.Type,
			Host:     provider.Host,
			Port:     provider.Port,
			Database: provider.Database,
			User:     provider.User,
			Password: provider.Password,
		}
	}

	for _, m := range conf.Maps {
		c.Maps[m.Name] = server.Map{
			MinZoom: m.MinZoom,
			MaxZoom: m.MaxZoom,
		}
	}

	for _, layer := range conf.Layers {
		c.Layers[layer.Name] = server.Layer{
			Provider: layer.Provider,
		}
	}

	return c
}
