//tegola server
package main

import "github.com/terranodo/tegola/server"

func main() {
	//	TODO: move port to conifg file
	server.Start(":9090")
}
