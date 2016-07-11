package server

import (
	"log"
	"net/http"
)

// Start starts the tile server binding to the provided port
func Start(port string) {
	//	notify the user the server is starting
	log.Printf("Starting tegola server on port %v\n", port)

	// Main page.
	http.Handle("/", http.FileServer(http.Dir("static")))
	// setup routes
	http.HandleFunc("/maps/", handleZXY)

	// TODO: make http port configurable
	log.Fatal(http.ListenAndServe(port, nil))
}
