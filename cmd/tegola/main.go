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
		Name  string
		Layer string
	}
	Layers []struct {
		Name      string
		Provider  string
		TableName string
		SQL       string
	}
}

//	hold parsed config from config file
var conf Config

func main() {
	//	open our config file
	f, err := os.Open("config.toml")
	if err != nil {
		log.Fatal("unable to open config file: ", err)
	}
	defer f.Close()

	//	unmarshal to our server config
	if err := toml.NewDecoder(f).Decode(&conf); err != nil {
		log.Fatal("config file error:", err)
	}

	//	setup our providers, maps and layers
	if err = server.Init(mapServerConf(conf)); err != nil {
		log.Fatal("server init error:", err)
	}

	//	bind our webserver
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
			Name:     provider.Name,
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
			Name:  m.Name,
			Layer: m.Layer,
		}
	}

	for _, layer := range conf.Layers {
		c.Layers[layer.Name] = server.Layer{
			Name:     layer.Name,
			Provider: layer.Provider,
			SQL:      layer.SQL,
		}
	}

	return c
}
