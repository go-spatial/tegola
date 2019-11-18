package simplify

import (
	"log"
	"os"
)

const debug = false

var logger *log.Logger

func init() {
	if debug {
		logger = log.New(os.Stderr, "simplify:", log.Lshortfile|log.LstdFlags)
	}
}
