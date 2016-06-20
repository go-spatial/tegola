package server

import (
	"log"
	"net/http"
)

//	starts the tile server binding to the provided port
func Start(port string) {
	//	notify the user the server is starting
	log.Printf("starting tegola server on port %v\n", port)

	//	setup routes
	http.HandleFunc("/maps/", handleZXY)

	//	TODO: make http port configurable
	log.Fatal(http.ListenAndServe(port, nil))
}
