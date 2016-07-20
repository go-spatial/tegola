//tegola server
package main

import (
	"flag"
	"log"
	"os"
	"text/template"

	"github.com/naoina/toml"

	"github.com/terranodo/tegola/server"
)

type Config struct {
	Webserver struct {
		Port      string
		LogFile   string
		LogFormat string
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
		Name   string
		Layers []struct {
			Name      string
			Provider  string
			Minzoom   int
			Maxzoom   int
			TableName string
			SQL       string
		}
	}
}

//	hold parsed config from config file
var conf Config

func main() {
	flag.Parse()
	//	open our config file
	f, err := os.Open("config.toml")
	if err != nil {
		log.Fatal("unable to open config file: ", err)
	}
	defer f.Close()

	//	unmarshal to our server config
	if err = toml.NewDecoder(f).Decode(&conf); err != nil {
		log.Fatal("config file error:", err)
	}

	// Command line logfile overrides config file.
	if logFile != "" {
		conf.Webserver.LogFile = logFile
		// Need to make sure that the log file exists.
	}
	if server.DefaultLogFormat != logFormat || conf.Webserver.LogFormat == "" {
		conf.Webserver.LogFormat = logFormat
	}

	//	setup our providers, maps and layers
	if err = server.Init(mapServerConf(conf)); err != nil {
		log.Fatal("server init error:", err)
	}

	//	bind our webserver
	server.Start(conf.Webserver.Port)
}

//	map our config file to our web server config
func mapServerConf(conf Config) server.Config {
	var c server.Config
	var err error

	if conf.Webserver.LogFile != "" {
		if c.LogFile, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666); err != nil {
			log.Printf("Unable to open logfile (%v) for writing: %v", logFile, err)
			os.Exit(2)
		}

	}
	if conf.Webserver.LogFormat == "" {
		conf.Webserver.LogFormat = server.DefaultLogFormat
	}
	c.LogTemplate = template.New("logfile")
	if _, err := c.LogTemplate.Parse(conf.Webserver.LogFormat); err != nil {
		log.Printf("Could not parse log template: %v error: %v", conf.Webserver.LogFormat, err)
		os.Exit(3)
	}

	//	iterate providers
	for _, provider := range conf.Providers {
		c.Providers = append(c.Providers, server.Provider{
			Name:     provider.Name,
			Type:     provider.Type,
			Host:     provider.Host,
			Port:     provider.Port,
			Database: provider.Database,
			User:     provider.User,
			Password: provider.Password,
		})
	}

	//	iterate maps
	for _, m := range conf.Maps {
		serverMap := server.Map{
			Name: m.Name,
		}

		//	iterate layers
		for _, l := range m.Layers {
			serverMap.Layers = append(serverMap.Layers, server.Layer{
				Name:      l.Name,
				Provider:  l.Provider,
				Minzoom:   l.Minzoom,
				Maxzoom:   l.Maxzoom,
				TableName: l.TableName,
				SQL:       l.SQL,
			})
		}

		c.Maps = append(c.Maps, serverMap)
	}

	return c
}
