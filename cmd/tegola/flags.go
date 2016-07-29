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

func init() {
	flag.StringVar(&configFile, "config-file", "config.toml", "Path to a toml config file. Can be absolute or relative.")
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
	flag.StringVar(&port, "port", ":8080", "Webserver port to bind to")
}
