package driver

import (
	"fmt"
	"log"
	"os"
)

const (
	logPrefix = "hdb.driver"
)

var dlog = log.New(os.Stderr, fmt.Sprintf("%s ", logPrefix), log.Ldate|log.Ltime|log.Lshortfile)
