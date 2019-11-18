package clip

import "log"

const debug = false

func init() {
	if debug {
		log.SetFlags(log.LstdFlags | log.Llongfile)
	}
}
