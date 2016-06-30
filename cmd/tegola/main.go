//tegola server
package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/naoina/toml"

	"github.com/terranodo/tegola/server"
)

func main() {
	//	open our config file
	f, err := os.Open("config.toml")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	//	read config file into memory
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	//	unmarshal to our server config
	if err := toml.Unmarshal(buf, &server.Conf); err != nil {
		panic(err)
	}

	log.Printf("config %+v\n", server.Conf)

	//	TODO: move port to conifg file
	server.Start(":8080")
}
