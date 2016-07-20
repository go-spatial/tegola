package server

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/terranodo/tegola/mvt"
)

//	incoming requests are associated with a map
var maps = map[string][]*mapLayer{}
var logFile *os.File
var logTemplate *template.Template

type logItem struct {
	RequestIP string
	Time      time.Time
	X         int
	Y         int
	Z         int
}

func Log(item logItem) {
	if logFile == nil {
		return
	}
	log.Println("Logging something")
	var l string
	if item.Time.IsZero() {
		item.Time = time.Now()
	}
	lbuf := bytes.NewBufferString(l)
	if err := logTemplate.Execute(lbuf, item); err != nil {
		// Don't care about the error.
		log.Println("Error writing log to log file.", err)
		return
	}
	b := lbuf.Bytes()
	// If there is no new line, let's add it.
	if b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	// Don't care about the error.
	logFile.Write(b)
}

//	map layers point to a provider
type mapLayer struct {
	Name     string
	Minzoom  int
	Maxzoom  int
	Provider mvt.Provider
}

// Start starts the tile server binding to the provided port
func Start(port string) {
	//	notify the user the server is starting
	log.Printf("Starting tegola server on port %v\n", port)

	//	setup routes
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/maps/", handleZXY)

	// TODO: make http port configurable
	log.Fatal(http.ListenAndServe(port, nil))
}
