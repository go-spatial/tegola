package main

import (
	"flag"

	"github.com/terranodo/tegola/server"
)

var (
	configFile string
	logFile    string
	logFormat  string
	port       string
)

const defaultHTTPPort = ":8080"

func init() {
	flag.StringVar(&configFile, "config", "config.toml", "Location of config file in TOML format. Can be absolute, relative or remote over http(s).")
	flag.StringVar(&logFile, "log-file", "", "The file to log output to. Disable by default.")
	flag.StringVar(&logFormat, "log-format", server.DefaultLogFormat,
		`The format that the logger will log with.
	
	Available fields:
		{{.Time}} : The current Date Time in RFC 2822 format.
		{{.RequestIP}} : The IP address of the the requester.
		{{.Z}} : The Zoom level.
		{{.X}} : The X Coordinate.
		{{.Y}} : The Y Coordinate.
`)
	flag.StringVar(&port, "port", defaultHTTPPort, "Webserver port to bind to")
}
