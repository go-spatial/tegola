package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/terranodo/tegola/server"
)

var (
	configFile = *flag.String("config", "config.toml", "")
	logFile    = *flag.String("log-file", "", "")
	logFormat  = *flag.String("log-format", server.DefaultLogFormat, "")
	port       = *flag.String("port", defaultHTTPPort, "")
)

const defaultHTTPPort = ":8080"

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `tegola %v <http://tegola.io>
MVT tile server with support for PostGIS

USAGE:	tegola [OPTIONS]

OPTIONS:
  -h, --help                Print usage
  -v, --version             Print version and quit
      --config      string  Location of config file in TOML format. Can be local or remote over http(s) (default config.toml)
      --port        string  Port to bind HTTP server to (example :8080)
      --log-file    string  The file to write request logs to (default disabled)
      --log-format  string  The format the logger will log with. Available fields:
                                {{.Time}} : The current Date Time in RFC 2822 format
                                {{.RequestIP}} : The IP address of the the requester
                                {{.Z}} : The Zoom level
                                {{.X}} : The X Coordinate
                                {{.Y}} : The Y Coordinate
  
`, Version)
	}
}
